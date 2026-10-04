package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/josecleiton/logn/backend/internal/domain"
	"github.com/josecleiton/logn/backend/internal/locale"
)

// otpSendWindow e otpSendPerSource são o balde de envio de código por IP. Vinte por hora
// folgam para uma sala de aula cadastrando atrás do mesmo NAT, e um IP sozinho fica
// longe do teto global (domain.OTPHourlySendCap).
const (
	otpSendWindow    = time.Hour
	otpSendPerSource = 20
)

type RequestOTPPayload struct {
	Email   string `json:"email"`
	Purpose string `json:"purpose"` // "verify_email", "reset_password"
}

// requestOTPHandler manda por e-mail o código de confirmar endereço ou redefinir senha.
//
//	@Summary		Pede um OTP por e-mail
//	@Description	O e-mail sai na língua de `Accept-Language`. O reenvio tem intervalo mínimo e teto por IP e global (`otp_resend_too_soon`, com `Retry-After`).
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		RequestOTPPayload	true	"E-mail e propósito (verify_email ou reset_password)"
//	@Success		200		{object}	object{status=string}	"otp_sent"
//	@Failure		400		{object}	apiError				"invalid_request, invalid_email"
//	@Failure		429		{object}	apiError				"otp_resend_too_soon, otp_locked, rate_limited"
//	@Failure		500		{object}	apiError				"internal"
//	@Router			/api/v1/auth/request-otp [post]
func (s *Server) requestOTPHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload RequestOTPPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}

	email, err := domain.NormalizeEmail(payload.Email)
	if err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidEmail)
		return
	}

	// For MVP, default purpose
	if payload.Purpose == "" {
		payload.Purpose = domain.OTPPurposeVerifyEmail
	}
	if !domain.ValidOTPPurpose(payload.Purpose) {
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}

	// Envio por balde de IP, por hora, na frente do teto global. Sem ele, um IP só
	// enchia o teto global em minutos e fechava cadastro e recuperação para todo mundo.
	// O código é o do reenvio: trava só o envio, e o código que já chegou vale.
	if s.otpSendLimiter != nil && !s.otpSendLimiter.allowIP(r) {
		w.Header().Set("Retry-After", strconv.Itoa(int(otpSendWindow.Seconds())))
		writeError(w, http.StatusTooManyRequests, codeOTPResendTooSoon)
		return
	}

	code := domain.GenerateOTP()

	// O e-mail sai na língua do app, que vem no Accept-Language. Ele entra na caixa de
	// saída junto com o código, e vai para a fila quando o pedido termina (ADR 0026).
	lang := locale.Negotiate(r)
	if err := s.repo.SaveOTP(r.Context(), email, code, payload.Purpose, lang, domain.OTPValidity); err != nil {
		if errors.Is(err, domain.ErrOTPCooldown) {
			w.Header().Set("Retry-After", "60")
			// Código distinto do limite por IP: este trava só o reenvio, e quem já
			// recebeu um código ainda pode digitá-lo.
			writeError(w, http.StatusTooManyRequests, codeOTPResendTooSoon)
			return
		}
		var locked *domain.OTPLockedError
		if errors.As(err, &locked) {
			w.Header().Set("Retry-After", strconv.Itoa(int(locked.RetryAfter.Seconds())+1))
			writeError(w, http.StatusTooManyRequests, codeOTPLocked)
			return
		}
		if errors.Is(err, domain.ErrOTPSendCapReached) {
			log.Printf("otp: teto de %d envios por hora atingido", domain.OTPHourlySendCap)
			w.Header().Set("Retry-After", "300")
			writeError(w, http.StatusTooManyRequests, codeOTPResendTooSoon)
			return
		}
		log.Printf("otp não gravado: purpose=%s erro=%v", payload.Purpose, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "otp_sent"})
}

type VerifyOTPPayload struct {
	Email   string `json:"email"`
	Code    string `json:"code"`
	Purpose string `json:"purpose"`
}

// verifyOTPHandler confere o código sem consumi-lo: quem consome é o cadastro ou a
// redefinição.
//
//	@Summary	Confere um OTP
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Param		otp	body		VerifyOTPPayload		true	"E-mail, código e propósito"
//	@Success	200	{object}	object{status=string}	"otp_verified"
//	@Failure	400	{object}	apiError				"invalid_request"
//	@Failure	401	{object}	apiError				"otp_invalid"
//	@Failure	429	{object}	apiError				"rate_limited"
//	@Router		/api/v1/auth/verify-otp [post]
func (s *Server) verifyOTPHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload VerifyOTPPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}

	if payload.Purpose == "" {
		payload.Purpose = domain.OTPPurposeVerifyEmail
	}

	email, err := domain.NormalizeEmail(payload.Email)
	if err != nil || !domain.ValidOTPPurpose(payload.Purpose) {
		writeError(w, http.StatusUnauthorized, codeOTPInvalid)
		return
	}

	valid, err := s.repo.CheckOTP(r.Context(), email, payload.Code, payload.Purpose)
	if err != nil || !valid {
		// Sem isto a causa some: código errado, expirado e e-mail inexistente
		// devolvem a mesma coisa, e não há como diagnosticar em dev. O e-mail fica
		// fora do log: é dado pessoal, e o propósito já basta para achar o fluxo.
		log.Printf("verify-otp recusado: purpose=%q valid=%v err=%v", payload.Purpose, valid, err)
		writeError(w, http.StatusUnauthorized, codeOTPInvalid)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "otp_verified"})
}
