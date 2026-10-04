package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/josecleiton/logn/backend/internal/domain"
	"github.com/josecleiton/logn/backend/internal/googleplay"
	"github.com/josecleiton/logn/backend/internal/infrastructure/cloudauth"
	"github.com/josecleiton/logn/backend/internal/infrastructure/email"
)

// Rotas internas (/api/v1/internal/*). Chegam direto na URL .run.app, fora da
// verificação de origem, e a defesa é o token OIDC do Google: assinatura, emissor,
// audiência e, desde a ADR 0021, a conta de serviço que o assinou. Cada rota aceita uma
// conta só, e variável faltando fecha a rota (403).
const (
	// A conta com que o Cloud Scheduler assina a purga.
	envSchedulerAccount = "CLOUD_SCHEDULER_SERVICE_ACCOUNT"
	// A conta de administração, em nome da qual quem opera emite o token (ADR 0021).
	envAdminAccount = "ADMIN_SERVICE_ACCOUNT"
)

// LicenseNotifier manda o aviso de revogação ou devolução manual. É o Mailer; os
// testes trocam.
type LicenseNotifier interface {
	SendLicenseNotice(toEmail string, kind email.LicenseNoticeKind, lang, trackName, reason string) error
}

// internalCaller confere o token da rota interna contra a conta esperada e devolve o
// e-mail dela. Responde e devolve `false` quando não passa.
func (s *Server) internalCaller(w http.ResponseWriter, r *http.Request, accountEnv string) (string, bool) {
	audience := os.Getenv("CLOUD_SCHEDULER_AUDIENCE")
	account := os.Getenv(accountEnv)
	if audience == "" || account == "" {
		http.Error(w, "Endpoint disabled, missing audience or service account config", http.StatusForbidden)
		return "", false
	}

	caller, err := s.cloudValidator.ValidateToken(r.Context(), r.Header.Get("Authorization"), audience)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return "", false
	}
	if !cloudauth.SameAccount(caller.Email, account) {
		// Token válido do Google, de outra conta: é o caso que a audiência sozinha deixava
		// passar. O e-mail de conta de serviço não é dado pessoal.
		log.Printf("rota interna recusada: path=%s conta=%s", r.URL.Path, caller.Email)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return "", false
	}
	return caller.Email, true
}

// purgeHandler é a rotina diária: apaga as contas excluídas que venceram o prazo, as
// inscrições não confirmadas da lista de espera, as contagens de login errado com janela
// vencida e os e-mails velhos da caixa de saída, e, com o Google Play ligado, revoga as
// compras anuladas e reconhece as pendentes. Quem chama é o Cloud Scheduler.
//
//	@Summary		Purga diária
//	@Description	Só com ID token OIDC do Google, da conta `CLOUD_SCHEDULER_SERVICE_ACCOUNT`. Erros saem em texto, não em `{code}`.
//	@Tags			internal
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	object{status=string,purged=int,waitlist_purged=int,login_attempts_purged=int,outbox_purged=int,play_voided=int,play_acknowledged=int}
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		403	{string}	string	"Forbidden"
//	@Failure		500	{string}	string	"Internal error"
//	@Router			/api/v1/internal/purge [post]
func (s *Server) purgeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, ok := s.internalCaller(w, r, envSchedulerAccount); !ok {
		return
	}

	purged, err := s.repo.PurgeDeletedAccounts(r.Context())
	if err != nil {
		log.Printf("expurgo interrompido depois de %d contas: erro=%v", purged, err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	log.Printf("expurgo ok: %d contas apagadas", purged)

	// As inscrições pendentes da lista de espera vencem no mesmo passo diário (ADR 0022).
	pending, err := s.repo.PurgePendingWaitlist(r.Context())
	if err != nil {
		log.Printf("expurgo da lista de espera falhou: erro=%v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	log.Printf("expurgo ok: %d inscrições pendentes apagadas", pending)

	// As contagens de tentativa de login vencidas saem no mesmo passo.
	attempts, err := s.repo.PurgeLoginAttempts(r.Context())
	if err != nil {
		log.Printf("expurgo das tentativas de login falhou: erro=%v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	log.Printf("expurgo ok: %d contagens de login apagadas", attempts)

	// A caixa de saída guarda uma semana, para depuração (ADR 0026).
	outbox, err := s.repo.PruneOutbox(r.Context())
	if err != nil {
		log.Printf("expurgo da caixa de saída falhou: erro=%v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	log.Printf("expurgo ok: %d e-mails da caixa de saída apagados", outbox)

	// Reembolsos e estornos do Google Play, no mesmo passo diário (ADR 0022). Eram um job
	// do Scheduler à parte, com a mesma conta; juntos, sobra um job do free tier. Erro da
	// loja responde 500 e o Scheduler tenta tudo de novo: os passos de cima não mudam
	// nada na segunda vez.
	voided, acked := 0, 0
	if s.play != nil {
		voided, acked, err = s.revokeVoidedPlay(r.Context())
		if err != nil {
			log.Printf("rotina do Google Play falhou: erro=%v", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
		log.Printf("rotina do Google Play ok: %d anuladas na janela, %d reconhecidas", voided, acked)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status": "ok", "purged": purged, "waitlist_purged": pending, "login_attempts_purged": attempts,
		"outbox_purged": outbox, "play_voided": voided, "play_acknowledged": acked,
	})
}

// playVoidedWindow é quanto para trás a consulta das compras anuladas olha. A API guarda
// 30 dias; um a menos de folga para o relógio. Revogar de novo o que já está revogado
// não muda nada, e a janela larga cobre os dias em que o job não rodou.
const playVoidedWindow = 29 * 24 * time.Hour

// revokeVoidedPlay revoga as compras do Google Play anuladas: reembolso, estorno e
// cancelamento (ADR 0022). Depois reconhece as compras que ficaram sem reconhecimento.
// Roda na rotina diária, dentro da purga; devolve quantas anuladas a janela tinha e
// quantas compras reconheceu.
//
// Compra anulada que não é nossa também entra em `revoked_transactions`: ela nunca vira
// licença depois.
func (s *Server) revokeVoidedPlay(ctx context.Context) (int, int, error) {
	voided, err := s.play.Voided(ctx, time.Now().Add(-playVoidedWindow))
	if err != nil {
		return 0, 0, fmt.Errorf("compras anuladas não lidas: %w", err)
	}
	for _, v := range voided {
		if v.PurchaseToken == "" {
			continue
		}
		reason := "refund"
		if v.Fraud() {
			reason = "fraud"
		}
		at := v.VoidedAtMs()
		if at == 0 {
			at = time.Now().UnixMilli()
		}
		if err := s.repo.RevokeTransaction(ctx, domain.ProviderGooglePlay,
			googleplay.TransactionKey(v.PurchaseToken), reason, at); err != nil {
			return len(voided), 0, fmt.Errorf("compra anulada não revogada: %w", err)
		}
	}

	// A compra gravada cujo reconhecimento falhou, e o app não mandou de novo, o Play
	// estorna em 3 dias. O job reconhece o que ficou para trás nesse prazo.
	acked, err := s.acknowledgePending(ctx)
	if err != nil {
		return len(voided), acked, fmt.Errorf("compras sem reconhecimento: %w", err)
	}
	return len(voided), acked, nil
}

// playAcknowledgeWindow é o prazo do Play para reconhecer, com um dia de folga.
const playAcknowledgeWindow = 4 * 24 * time.Hour

// acknowledgePending reconhece as compras do Play dos últimos dias que ainda estão sem
// reconhecimento, e devolve quantas. Compra que não vale mais (anulada, cancelada) fica
// como está: reconhecer não a faz valer, e a revogação já cuidou dela.
func (s *Server) acknowledgePending(ctx context.Context) (int, error) {
	refs, err := s.repo.RecentPlayPurchases(ctx, time.Now().Add(-playAcknowledgeWindow))
	if err != nil {
		return 0, err
	}
	acked := 0
	for _, ref := range refs {
		p, err := s.play.Product(ctx, ref.ProductID, ref.PurchaseToken)
		if errors.Is(err, googleplay.ErrNotFound) {
			continue
		}
		if err != nil {
			return acked, err
		}
		if _, err := p.Environment(); err != nil || !p.NeedsAcknowledge() {
			continue
		}
		if err := s.acknowledgePlay(ctx, ref.ProductID, ref.PurchaseToken); err != nil {
			return acked, err
		}
		acked++
	}
	return acked, nil
}

// licenseActionRequest é o corpo de revoke e appeal. `reason` só na revogação,
// `outcome` só na contestação.
type licenseActionRequest struct {
	UserID   string `json:"user_id"`
	TrackID  string `json:"track_id"`
	Reason   string `json:"reason"`
	Outcome  string `json:"outcome"`
	Evidence string `json:"evidence"`
}

// revokeLicenseHandler revoga à mão a licença de uma conta e avisa por e-mail (termos,
// seção 10.5; ADR 0021).
//
//	@Summary		Revoga uma licença à mão
//	@Description	Só com ID token OIDC do Google, da conta `ADMIN_SERVICE_ACCOUNT` (ADR 0021). Campo desconhecido no corpo é 400.
//	@Tags			internal
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			action	body		licenseActionRequest	true	"Conta, trilha e motivo"
//	@Success		200		{object}	object{status=string,reason=string,outcome=string,locale=string,email_sent=bool}
//	@Failure		400		{object}	apiError	"invalid_request"
//	@Failure		401		{string}	string		"Unauthorized"
//	@Failure		403		{string}	string		"Forbidden"
//	@Failure		409		{object}	apiError	"license_not_active, license_store_revoked"
//	@Failure		429		{object}	apiError	"rate_limited"
//	@Failure		500		{object}	apiError	"internal"
//	@Failure		503		{object}	apiError	"internal (sem mailer)"
//	@Router			/api/v1/internal/licenses/revoke [post]
func (s *Server) revokeLicenseHandler(w http.ResponseWriter, r *http.Request) {
	s.manualLicenseAction(w, r, true)
}

// appealLicenseHandler responde à contestação de uma revogação manual, muda a licença
// como a seção 10.5 manda e avisa por e-mail.
//
//	@Summary		Responde à contestação de uma revogação
//	@Description	Só com ID token OIDC do Google, da conta `ADMIN_SERVICE_ACCOUNT` (ADR 0021). Campo desconhecido no corpo é 400.
//	@Tags			internal
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			action	body		licenseActionRequest	true	"Conta, trilha e desfecho"
//	@Success		200		{object}	object{status=string,reason=string,outcome=string,locale=string,email_sent=bool}
//	@Failure		400		{object}	apiError	"invalid_request"
//	@Failure		401		{string}	string		"Unauthorized"
//	@Failure		403		{string}	string		"Forbidden"
//	@Failure		409		{object}	apiError	"license_not_revoked, license_appeal_out_of_order, license_store_revoked"
//	@Failure		429		{object}	apiError	"rate_limited"
//	@Failure		500		{object}	apiError	"internal"
//	@Failure		503		{object}	apiError	"internal (sem mailer)"
//	@Router			/api/v1/internal/licenses/appeal [post]
func (s *Server) appealLicenseHandler(w http.ResponseWriter, r *http.Request) {
	s.manualLicenseAction(w, r, false)
}

func (s *Server) manualLicenseAction(w http.ResponseWriter, r *http.Request, revoke bool) {
	actor, ok := s.internalCaller(w, r, envAdminAccount)
	if !ok {
		return
	}
	// Sem como avisar, nada muda: a seção 10.5 promete o e-mail logo depois.
	if s.licenseNotifier == nil {
		log.Printf("licença manual recusada: e-mail não configurado")
		writeError(w, http.StatusServiceUnavailable, codeInternal)
		return
	}

	var req licenseActionRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	if !isUUID(req.UserID) || !isUUID(req.TrackID) {
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}

	action := domain.ManualLicenseAction{
		UserID: req.UserID, TrackID: req.TrackID, Reason: req.Reason,
		Outcome: req.Outcome, Evidence: req.Evidence, Actor: actor,
	}
	var notice domain.LicenseNotice
	var err error
	if revoke {
		notice, err = s.repo.RevokeManually(r.Context(), action)
	} else {
		notice, err = s.repo.AnswerAppeal(r.Context(), action)
	}
	switch {
	case errors.Is(err, domain.ErrInvalidLicenseAction):
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	case errors.Is(err, domain.ErrNoActiveEntitlement):
		writeError(w, http.StatusConflict, codeLicenseNotActive)
		return
	case errors.Is(err, domain.ErrNotManuallyRevoked):
		writeError(w, http.StatusConflict, codeLicenseNotRevoked)
		return
	case errors.Is(err, domain.ErrAppealOutOfOrder):
		writeError(w, http.StatusConflict, codeLicenseAppealOutOfOrder)
		return
	case errors.Is(err, domain.ErrStillRevokedByStore):
		writeError(w, http.StatusConflict, codeLicenseStoreRevoked)
		return
	case err != nil:
		log.Printf("licença manual não gravada: user=%s track=%s revogar=%t erro=%v", req.UserID, req.TrackID, revoke, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}

	kind := noticeKind(notice)
	// A mudança já está gravada. E-mail que falha não a desfaz: a resposta diz, e quem
	// opera avisa por outro caminho. O endereço não vai para o log.
	emailed := true
	if err := s.licenseNotifier.SendLicenseNotice(notice.Email, kind, notice.Locale, notice.TrackName, notice.Reason); err != nil {
		emailed = false
		log.Printf("aviso de licença não enviado: user=%s track=%s erro=%v", req.UserID, req.TrackID, err)
	}
	log.Printf("licença manual: user=%s track=%s revogar=%t motivo=%s resultado=%s por=%s aviso=%t",
		req.UserID, req.TrackID, revoke, notice.Reason, notice.Outcome, actor, emailed)

	status := "appeal_" + notice.Outcome
	if revoke {
		status = "revoked"
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status": status, "reason": notice.Reason, "outcome": notice.Outcome,
		"locale": notice.Locale, "email_sent": emailed,
	})
}

// noticeKind é o aviso de cada passo; resultado vazio é a revogação.
func noticeKind(n domain.LicenseNotice) email.LicenseNoticeKind {
	switch n.Outcome {
	case domain.AppealReceived:
		return email.LicenseAppealReceived
	case domain.AppealReview:
		return email.LicenseUnderReview
	case domain.AppealAccepted:
		if n.StoreRevoked {
			return email.LicenseAcceptedButRefunded
		}
		return email.LicenseRestored
	case domain.AppealRejected:
		return email.LicenseRevokedAfterReview
	default:
		return email.LicenseRevoked
	}
}
