package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/josecleiton/logn/backend/internal/domain"
	"github.com/josecleiton/logn/backend/internal/infrastructure/socialauth"
	"github.com/josecleiton/logn/backend/internal/legal"
	"github.com/josecleiton/logn/backend/internal/locale"
)

// socialVerifiersFromEnv monta um verificador por provedor configurado.
//
// Sem GOOGLE_IOS_CLIENT_ID o login pelo Google fica desligado, e a rota responde
// `provider_disabled`; o servidor sobe do mesmo jeito. O client ID não é segredo, e
// derrubar o deploy por ele tiraria do ar também quem entra por e-mail.
func socialVerifiersFromEnv() map[string]socialauth.Verifier {
	verifiers := map[string]socialauth.Verifier{}
	if aud := strings.TrimSpace(os.Getenv("GOOGLE_IOS_CLIENT_ID")); aud != "" {
		v, err := socialauth.NewGoogleVerifier(aud)
		if err != nil {
			log.Fatalf("GOOGLE_IOS_CLIENT_ID inválido: %v", err)
		}
		verifiers[socialauth.ProviderGoogle] = v
	} else {
		log.Println("GOOGLE_IOS_CLIENT_ID is not set. Google sign-in is disabled.")
	}
	return verifiers
}

type SocialLoginRequest struct {
	Provider string `json:"provider"`
	IDToken  string `json:"id_token"`
	// O nonce cru. O pedido ao provedor levou o SHA-256 dele, e é isso que o token traz.
	Nonce string `json:"nonce"`

	// Só na primeira vez, quando a identidade ainda não tem conta: o mesmo que o
	// cadastro por e-mail pede.
	AgeConfirmed     bool                     `json:"age_confirmed"`
	Country          string                   `json:"country"`
	LegalAcceptances []domain.LegalAcceptance `json:"legal_acceptances"`
}

// verifySocial confere o token com o verificador do provedor e responde o erro, se
// houver. `ok` falso quer dizer que a resposta já foi escrita.
func (s *Server) verifySocial(ctx context.Context, w http.ResponseWriter, provider, idToken, nonce string) (socialauth.Identity, bool) {
	verifier, found := s.social[provider]
	if !found {
		if provider == socialauth.ProviderGoogle {
			writeError(w, http.StatusServiceUnavailable, codeProviderDisabled)
		} else {
			writeError(w, http.StatusBadRequest, codeInvalidRequest)
		}
		return socialauth.Identity{}, false
	}
	id, err := verifier.Verify(ctx, idToken, nonce)
	if err != nil {
		// O motivo fica no log, sem o token. Para o app é um código só: dizer qual
		// checagem falhou ensina a montar o próximo token.
		log.Printf("token social recusado: provider=%s erro=%v", provider, err)
		writeError(w, http.StatusUnauthorized, codeSocialTokenInvalid)
		return socialauth.Identity{}, false
	}
	return id, true
}

// socialLoginHandler entra com o ID token de um provedor externo (ADR 0016).
//
// A conta é achada pela identidade (`provider`, `sub`). Na primeira vez, sem identidade
// ligada, vale o e-mail: se já existe conta com ele, a identidade é ligada a ela, desde
// que o provedor diga que o e-mail foi verificado. Sem conta nenhuma, responde
// `signup_required` até o pedido trazer idade e aceite, e então cria a conta sem senha.
func (s *Server) socialLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SocialLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}

	ctx := r.Context()
	id, ok := s.verifySocial(ctx, w, req.Provider, req.IDToken, req.Nonce)
	if !ok {
		return
	}

	userID, err := s.repo.GetUserIDByIdentity(ctx, req.Provider, id.Subject)
	if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
		log.Printf("identidade não lida: provider=%s erro=%v", req.Provider, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}

	if userID == "" {
		// Daqui em diante o e-mail decide, e só e-mail verificado decide alguma coisa:
		// sem isso, qualquer um criava no provedor uma conta com o endereço de outra
		// pessoa e entrava na conta dela aqui.
		if !id.EmailVerified {
			writeError(w, http.StatusForbidden, codeSocialEmailUnverified)
			return
		}
		email, err := domain.NormalizeEmail(id.Email)
		if err != nil {
			writeError(w, http.StatusForbidden, codeSocialEmailUnverified)
			return
		}

		existing, err := s.repo.GetUserByEmail(ctx, email)
		if errors.Is(err, pgx.ErrNoRows) {
			s.socialSignup(w, r, req, id, email)
			return
		}
		if err != nil {
			log.Printf("conta não lida no login social: erro=%v", err)
			writeError(w, http.StatusInternalServerError, codeInternal)
			return
		}
		userID, err = s.repo.LinkIdentity(ctx, req.Provider, id.Subject, existing.ID)
		if err != nil {
			log.Printf("identidade não ligada: user=%s erro=%v", existing.ID, err)
			writeError(w, http.StatusInternalServerError, codeInternal)
			return
		}
		log.Printf("identidade ligada: provider=%s user=%s", req.Provider, userID)
	}

	// Entrar dentro da carência cancela a exclusão, como no login por senha.
	restored, err := s.repo.CancelAccountDeletion(ctx, userID)
	if err != nil {
		log.Printf("exclusão não cancelada no login social: user=%s erro=%v", userID, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}
	if restored {
		log.Printf("exclusão cancelada pelo login social: user=%s", userID)
	}

	// O e-mail da sessão é o da conta, não o do token: o do provedor pode ter mudado
	// depois do vínculo, e a corrida no vínculo pode ter levado a outra conta.
	account, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		log.Printf("conta não lida no login social: user=%s erro=%v", userID, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}

	s.issueSession(ctx, w, userID, account.Email, restored)
}

// socialSignup cria a conta de quem entrou pelo provedor pela primeira vez. As regras
// de idade e aceite são as do cadastro por e-mail; o OTP não entra, porque o provedor
// já provou o e-mail.
func (s *Server) socialSignup(w http.ResponseWriter, r *http.Request, req SocialLoginRequest, id socialauth.Identity, email string) {
	ctx := r.Context()

	// Primeira tentativa, sem nada do cadastro: o app ainda não mostrou a tela de
	// idade e termos. O mesmo token volta no próximo pedido, com eles.
	if !req.AgeConfirmed && len(req.LegalAcceptances) == 0 {
		writeError(w, http.StatusConflict, codeSignupRequired)
		return
	}

	country, ok := legal.NormalizeCountry(req.Country)
	if !ok {
		writeError(w, http.StatusBadRequest, codeInvalidCountry)
		return
	}
	if !req.AgeConfirmed {
		writeError(w, http.StatusBadRequest, codeAgeNotConfirmed)
		return
	}
	current, err := s.currentLegalVersions(ctx)
	if err != nil {
		log.Printf("cadastro social sem versões legais: erro=%v", err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}
	acceptances, err := checkLegalAcceptances(req.LegalAcceptances, current)
	if errors.Is(err, errLegalOutdated) {
		writeError(w, http.StatusConflict, codeLegalVersionOutdated)
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, codeLegalAcceptanceRequired)
		return
	}

	userID, err := s.repo.CreateSocialUser(ctx, email, req.Provider, id.Subject, req.AgeConfirmed, country, acceptances)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		// Outro pedido criou a conta ou ligou a identidade no meio. O app tenta de
		// novo e cai no caminho da conta existente.
		log.Printf("cadastro social em corrida: provider=%s", req.Provider)
		writeError(w, http.StatusConflict, codeEmailTaken)
		return
	}
	if err != nil {
		log.Printf("cadastro social não criado: provider=%s erro=%v", req.Provider, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}

	lang := locale.Negotiate(r)
	go func(email, lang string) {
		if err := s.mailer.SendWelcome(email, lang); err != nil {
			log.Printf("boas-vindas não enviadas: user=%s erro=%v", userID, err)
		}
	}(email, lang)

	s.issueSession(ctx, w, userID, email, false)
}
