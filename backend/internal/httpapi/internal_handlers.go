package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/josecleiton/logn/backend/internal/domain"
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"status": "ok", "purged": purged})
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
func (s *Server) revokeLicenseHandler(w http.ResponseWriter, r *http.Request) {
	s.manualLicenseAction(w, r, true)
}

// appealLicenseHandler responde à contestação de uma revogação manual, muda a licença
// como a seção 10.5 manda e avisa por e-mail.
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
