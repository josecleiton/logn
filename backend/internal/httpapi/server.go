// Package httpapi é a camada HTTP do backend: rotas, handlers e middleware.
//
// Quem monta o servidor (o `main`) entrega as dependências prontas em `Deps` e recebe
// um `http.Handler`. Regra de negócio não mora aqui: o handler lê o pedido, tira o
// `user_id` do token, chama `internal/domain` e responde por `writeError` (ADR 0018).
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/josecleiton/logn/backend/internal/domain"
	"github.com/josecleiton/logn/backend/internal/infrastructure/cloudauth"
	"github.com/josecleiton/logn/backend/internal/infrastructure/email"
	"github.com/josecleiton/logn/backend/internal/infrastructure/socialauth"
	"github.com/josecleiton/logn/backend/internal/locale"
	"github.com/josecleiton/logn/backend/internal/storekit"
)

type Server struct {
	repo           *domain.Repository
	mailer         *email.Mailer
	cloudValidator cloudauth.Validator
	storekit       *storekit.Validator
	// A Google Play Developer API (ADR 0022). Nula, a compra pelo Play responde
	// `store_unavailable`.
	play PlayVerifier
	// O aviso da revogação manual; é o mailer, trocado nos testes.
	licenseNotifier LicenseNotifier
	// Um verificador por provedor de login ligado. Provedor fora do mapa está desligado.
	social map[string]socialauth.Verifier
	// Provedores que revogam o acesso na exclusão da conta (a Apple, o GitHub). Quem
	// está aqui exige o `authorization_code` na exclusão confirmada por ele.
	revokers map[string]socialauth.Revoker
	// Troca o código do GitHub pelo bilhete. Nulo com o GitHub desligado.
	github GitHubExchanger
	// Lista de espera do iPhone (ADR 0022). Nula, as rotas não existem.
	waitlist       *WaitlistConfig
	waitlistMailer WaitlistMailer
	// Sync por conta, além do limite por IP da rota. Nulo nos testes que chamam o
	// handler direto.
	syncUserLimiter *rateLimiter
	// Envio de código por IP, por hora (otpSendPerSource). Nulo nos testes que chamam o
	// handler direto.
	otpSendLimiter *rateLimiter
}

// syncSlots limita quantos syncs decodificam e gravam ao mesmo tempo. O corpo vira
// memória só depois de pegar a vaga, e o pico fica em vagas × syncBodyLimit (mais a
// cópia do decode), qualquer que seja o tráfego: a instância tem 256 MiB.
var syncSlots = make(chan struct{}, 8)

// syncSlotWait é quanto um sync espera pela vaga antes de ouvir 429 (variável só para o
// teste), e syncBodyReadTimeout, quanto o corpo tem para chegar depois dela. O lote do
// app tem menos de 100 KB.
var syncSlotWait = 5 * time.Second

const syncBodyReadTimeout = 10 * time.Second

// GitHubExchanger é a troca do código do GitHub (ADR 0019), trocada nos testes.
type GitHubExchanger interface {
	Exchange(ctx context.Context, code, verifier, nonceHash string, keepToken bool) (socialauth.Exchanged, error)
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (s *Server) readyHandler(w http.ResponseWriter, r *http.Request) {
	if err := s.repo.Ping(r.Context()); err != nil {
		http.Error(w, "Database not ready", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ready"))
}

func (s *Server) pingHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "LogN Backend is running!",
	})
}

// authenticate devolve o dono do token do cabeçalho `Authorization: Bearer`.
//
// Responde 401 e devolve `false` quando não há token válido — o chamador só precisa
// desistir.
func (s *Server) authenticate(w http.ResponseWriter, r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		writeError(w, http.StatusUnauthorized, codeUnauthenticated)
		return "", false
	}

	userID, err := domain.UserIDFromAccessToken(strings.TrimPrefix(header, "Bearer "))
	if err != nil {
		writeError(w, http.StatusUnauthorized, codeUnauthenticated)
		return "", false
	}

	// Ensure user exists and has not requested deletion
	if !s.repo.IsUserActive(r.Context(), userID) {
		writeError(w, http.StatusUnauthorized, codeUnauthenticated)
		return "", false
	}

	return userID, true
}

// optionalAccount é o dono do token, quando há token; sem cabeçalho, é o visitante ("").
// Token presente e inválido responde 401 e devolve `false`, para o app renovar a sessão
// em vez de receber a resposta de quem não entrou.
func (s *Server) optionalAccount(w http.ResponseWriter, r *http.Request) (string, bool) {
	if r.Header.Get("Authorization") == "" {
		return "", true
	}
	return s.authenticate(w, r)
}

func (s *Server) syncHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	if s.syncUserLimiter != nil && !s.syncUserLimiter.allow(userID) {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, codeRateLimited)
		return
	}

	// A espera pela vaga tem fim: fila sem fim só empilha conexões. E quem pega a vaga
	// tem pouco tempo para mandar o corpo: o `ReadTimeout` do servidor é de 30 s, e um
	// corpo pingado devagar segurava a vaga esse tempo todo, oito vezes, para todo mundo.
	wait := time.NewTimer(syncSlotWait)
	defer wait.Stop()
	select {
	case syncSlots <- struct{}{}:
		defer func() { <-syncSlots }()
	case <-wait.C:
		w.Header().Set("Retry-After", "5")
		writeError(w, http.StatusTooManyRequests, codeRateLimited)
		return
	case <-r.Context().Done():
		return
	}
	// Todo writer no caminho tem de deixar o ResponseController chegar à conexão
	// (`Unwrap`, como o do gzip). Sem isso o prazo some calado, e o corpo pingado volta
	// a segurar a vaga pelos 30 s do `ReadTimeout`.
	if err := http.NewResponseController(w).SetReadDeadline(time.Now().Add(syncBodyReadTimeout)); err != nil {
		log.Printf("sync sem prazo de leitura do corpo: erro=%v", err)
	}

	var payload domain.SyncPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	if err := domain.CheckSyncShape(payload); err != nil {
		log.Printf("sync recusado: user=%s eventos=%d motivo=%v", userID, len(payload.Events), err)
		if errors.Is(err, domain.ErrSyncTooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, codeSyncTooLarge)
			return
		}
		writeError(w, http.StatusForbidden, codeSyncRejected)
		return
	}

	// O corpo do pedido não decide de quem é a cadeia. Mandava e o servidor obedecia:
	// dava para escrever eventos na conta de qualquer um.
	payload.UserID = userID

	ctx := r.Context()

	serverLastHash, err := s.repo.GetUserLastHash(ctx, payload.UserID)
	if err != nil {
		log.Printf("sync sem estado: user=%s erro=%v", payload.UserID, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}

	valid, err := domain.ValidateSync(payload, serverLastHash)
	if err != nil {
		if err.Error() == "force_rebase" {
			log.Printf("sync rebase: user=%s eventos=%d topo_servidor=%s primeiro_previous=%s",
				payload.UserID, len(payload.Events), serverLastHash, payload.Events[0].PreviousHash)
			writeRebaseRequired(w, serverLastHash)
			return
		}

		// Sem este log, um sync recusado some: o cliente só vê o número do status e
		// o servidor não conta o motivo a ninguém.
		log.Printf("sync recusado: user=%s eventos=%d motivo=%v", payload.UserID, len(payload.Events), err)
		writeError(w, http.StatusForbidden, codeSyncRejected)
		return
	}

	if !valid {
		log.Printf("sync recusado: user=%s cadeia inválida", payload.UserID)
		writeError(w, http.StatusForbidden, codeSyncRejected)
		return
	}

	// Topo da cadeia depois deste sync. Devolver o topo antigo faria o cliente
	// continuar encadeando a partir de onde o servidor já não está.
	newTop := serverLastHash
	if len(payload.Events) > 0 {
		newTop = payload.Events[len(payload.Events)-1].CurrentHash
		if err := s.repo.InsertSyncEvents(ctx, payload, serverLastHash, newTop); err != nil {
			if errors.Is(err, domain.ErrStaleChain) {
				// Outro sync do mesmo usuário gravou entre a leitura e esta escrita.
				// O topo que ele deixou é o ponto de onde o cliente refaz a fila.
				top, topErr := s.repo.GetUserLastHash(ctx, payload.UserID)
				if topErr == nil {
					log.Printf("sync concorrente: user=%s eventos=%d topo=%s", payload.UserID, len(payload.Events), top)
					writeRebaseRequired(w, top)
					return
				}
				err = topErr
			}
			log.Printf("sync não gravado: user=%s eventos=%d erro=%v", payload.UserID, len(payload.Events), err)
			writeError(w, http.StatusInternalServerError, codeInternal)
			return
		}
		log.Printf("sync ok: user=%s eventos=%d topo=%s", payload.UserID, len(payload.Events), newTop)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "success",
		"events_applied": len(payload.Events),
		"new_top":        newTop,
	})
}

func writeRebaseRequired(w http.ResponseWriter, serverTop string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusConflict)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":           codeRebaseRequired,
		"status":         "rebase_required",
		"server_top":     serverTop,
		"events_applied": 0,
	})
}

// Método filtrado pela rota, como em getNodesHandler: HEAD tem de passar.
func (s *Server) challengesHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.optionalAccount(w, r)
	if !ok {
		return
	}
	lang := locale.Negotiate(r)
	challenges, err := s.repo.GetChallenges(r.Context(), lang, userID)
	if err != nil {
		log.Printf("desafios não lidos: locale=%s erro=%v", lang, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}

	writeAccountContentJSON(w, lang, userID, challenges)
}

// writeContentJSON responde conteúdo da trilha numa língua. `Content-Language` diz ao
// app em que língua o que chegou está — é o que ele grava junto da cópia offline —, e
// `Vary` impede um cache no caminho de servir o espanhol a quem pediu português.
func writeContentJSON(w http.ResponseWriter, lang string, body any) {
	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("Content-Language", lang)
	h.Add("Vary", "Accept-Language")
	h.Set("Cache-Control", "no-cache")
	json.NewEncoder(w).Encode(body)
}

// writeAccountContentJSON responde conteúdo que depende da conta: o visitante recebe o
// de todos, e a conta recebe também o que só ela vê — a trilha comprada, a indisponível
// que ela testa (ADR 0014). Essa não pode ficar num cache no caminho para servir a outra
// pessoa.
func writeAccountContentJSON(w http.ResponseWriter, lang, userID string, body any) {
	if userID == "" {
		writeContentJSON(w, lang, body)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("Content-Language", lang)
	h.Set("Cache-Control", "private, no-store")
	json.NewEncoder(w).Encode(body)
}
