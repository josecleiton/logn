package main

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/josecleiton/logn/backend/internal/domain"
)

// Códigos de erro da API. São o contrato com o app: ele decide e traduz por eles, pelo
// próprio catálogo. O servidor nunca devolve frase para a tela — ele não sabe em que
// língua o app está. Um código publicado não muda de nome; erro novo ganha código novo.
const (
	codeInvalidRequest          = "invalid_request"
	codeInvalidEmail            = "invalid_email"
	codePasswordTooShort        = "password_too_short"
	codePasswordTooLong         = "password_too_long"
	codeInvalidCountry          = "invalid_country"
	codeAgeNotConfirmed         = "age_not_confirmed"
	codeLegalAcceptanceRequired = "legal_acceptance_required"
	codeLegalVersionOutdated    = "legal_version_outdated"
	codeInvalidCredentials      = "invalid_credentials"
	codeOTPInvalid              = "otp_invalid"
	codeSessionInvalid          = "session_invalid"
	codeUnauthenticated         = "unauthenticated"
	codeEmailTaken              = "email_taken"
	codeRebaseRequired          = "rebase_required"
	codeSyncRejected            = "sync_rejected"
	codeOTPResendTooSoon        = "otp_resend_too_soon"
	codeRateLimited             = "rate_limited"
	codeOriginNotVerified       = "origin_not_verified"
	codeInternal                = "internal"
)

// apiError é o corpo de todo erro da API. `message` é para quem lê log e curl, em
// inglês, e o app nunca a mostra.
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeError responde um erro com código estável. Cabeçalhos já definidos, como o
// `Retry-After` de um 429, seguem valendo.
//
// Antes cada rota respondia texto solto ("Invalid or expired OTP", "Bad request"), e o
// app só tinha o status para decidir — e num ponto comparava o começo do texto.
func writeError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(apiError{Code: code, Message: http.StatusText(status)})
}

// passwordErrorCode leva o erro de ValidatePassword ao código da API.
func passwordErrorCode(err error) string {
	if errors.Is(err, domain.ErrPasswordTooLong) {
		return codePasswordTooLong
	}
	return codePasswordTooShort
}
