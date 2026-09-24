package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/josecleiton/logn/backend/internal/domain"
)

type RequestOTPPayload struct {
	Email   string `json:"email"`
	Purpose string `json:"purpose"` // "verify_email", "reset_password"
}

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

	code := domain.GenerateOTP()

	if err := s.repo.SaveOTP(r.Context(), email, code, payload.Purpose, 15*time.Minute); err != nil {
		if errors.Is(err, domain.ErrOTPCooldown) {
			w.Header().Set("Retry-After", "60")
			// Código distinto do limite por IP: este trava só o reenvio, e quem já
			// recebeu um código ainda pode digitá-lo.
			writeError(w, http.StatusTooManyRequests, codeOTPResendTooSoon)
			return
		}
		log.Printf("otp não gravado: purpose=%s erro=%v", payload.Purpose, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}

	// Send Email asynchronously
	go func(email, purpose, otp string) {
		if err := s.mailer.SendOTP(email, purpose, otp); err != nil {
			log.Printf("otp não enviado: purpose=%s erro=%v", purpose, err)
		}
	}(email, payload.Purpose, code)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "otp_sent"})
}

type VerifyOTPPayload struct {
	Email   string `json:"email"`
	Code    string `json:"code"`
	Purpose string `json:"purpose"`
}

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
