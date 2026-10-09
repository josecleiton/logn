package httpapi

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
	// Sync com mais eventos que domain.MaxSyncEvents. O app manda em lotes menores.
	codeSyncTooLarge = "sync_too_large"
	// Senha errada demais para o e-mail na janela (domain.LoginMaxAttempts). Trava só o
	// login por senha; o resto das rotas de conta segue aberto.
	codeLoginLocked = "login_locked"
	// Código errado demais para o e-mail e propósito na janela
	// (domain.OTPMaxFailuresPerWindow): nenhum código novo sai até ela vencer.
	codeOTPLocked = "otp_locked"
	codeOTPResendTooSoon        = "otp_resend_too_soon"
	codeRateLimited             = "rate_limited"
	codeOriginNotVerified       = "origin_not_verified"
	codeInternal                = "internal"

	// Login por provedor externo (ADR 0016).
	codeProviderDisabled      = "provider_disabled"
	codeSocialTokenInvalid    = "social_token_invalid"
	codeSocialEmailUnverified = "social_email_unverified"
	// A identidade não tem conta: o app pede idade e aceite e manda o pedido de novo.
	codeSignupRequired = "signup_required"
	// Excluir a conta pela senha não vale quando ela também entra por provedor que
	// exige revogar o acesso (a Apple): a confirmação tem de passar por ele.
	codeProviderReauthRequired = "provider_reauth_required"

	// Trilhas pagas.
	codePurchaseInvalid             = "purchase_invalid"
	codePurchaseAccountMismatch     = "purchase_account_mismatch"
	codePurchaseOwnedByOtherAccount = "purchase_owned_by_other_account"
	codePurchaseRevoked             = "purchase_revoked"
	codeUnknownProduct              = "unknown_product"
	codeEntitlementRequired         = "entitlement_required"
	codeDeviceIDRequired            = "device_id_required"
	// Compra do Google Play ainda não paga (boleto, dinheiro): o app espera e manda de
	// novo quando a loja avisar que pagou (ADR 0022).
	codePurchasePending = "purchase_pending"
	// A loja do pedido não está ligada neste servidor.
	codeStoreUnavailable = "store_unavailable"

	// Revogação manual (ADR 0021). Quem lê é o `just revoke`, não o app.
	codeLicenseNotActive        = "license_not_active"
	codeLicenseNotRevoked       = "license_not_revoked"
	codeLicenseAppealOutOfOrder = "license_appeal_out_of_order"
	codeLicenseStoreRevoked     = "license_store_revoked"

	// Apelido do placar. Os três primeiros o jogador corrige; o último diz que a conta
	// já escolheu ou perdeu a chance.
	codeNicknameInvalid  = "nickname_invalid"
	codeNicknameReserved = "nickname_reserved"
	codeNicknameTaken    = "nickname_taken"
	codeNicknameLocked   = "nickname_locked"

	// Corpo acima do teto da rota. As rotas antigas respondem 400 nesse caso, porque o
	// corpo cortado falha no decode; as do placar respondem 413.
	codeBodyTooLarge = "body_too_large"

	// Moderação do placar. Quem lê é o `just leaderboard-*`, não o app.
	codeUserNotFound        = "user_not_found"
	codeLeaderboardNoChange = "leaderboard_no_change"
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

// writeUnavailable responde 503 quando o banco não respondeu a uma checagem de sessão.
// Não pode sair 401: o app leria sessão inválida e deslogaria o jogador por um pool cheio.
func writeUnavailable(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "5")
	writeError(w, http.StatusServiceUnavailable, codeInternal)
}

// writeArgonBusy responde a comparação de senha que não achou vaga a tempo, como o sync
// sem vaga: 429 com `Retry-After`, e o app espera para tentar de novo. Não é credencial
// errada: 401 aqui diria ao jogador que errou a senha que ele digitou certo.
func writeArgonBusy(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "5")
	writeError(w, http.StatusTooManyRequests, codeRateLimited)
}

// passwordErrorCode leva o erro de ValidatePassword ao código da API.
func passwordErrorCode(err error) string {
	if errors.Is(err, domain.ErrPasswordTooLong) {
		return codePasswordTooLong
	}
	return codePasswordTooShort
}
