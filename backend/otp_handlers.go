package main

import (
	"context"
	"encoding/json"
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
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.Email == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// For MVP, default purpose
	if payload.Purpose == "" {
		payload.Purpose = "verify_email"
	}

	ctx := context.Background()
	code := domain.GenerateOTP()

	if err := s.repo.SaveOTP(ctx, payload.Email, code, payload.Purpose, 15*time.Minute); err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Send Email asynchronously
	go func(email, purpose, otp string) {
		if err := s.mailer.SendOTP(email, purpose, otp); err != nil {
			// Log error via telemetry/logger in real prod app
			_ = err 
		}
	}(payload.Email, payload.Purpose, code)

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
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if payload.Purpose == "" {
		payload.Purpose = "verify_email"
	}

	ctx := context.Background()
	valid, err := s.repo.CheckOTP(ctx, payload.Email, payload.Code, payload.Purpose)
	if err != nil || !valid {
		http.Error(w, "Invalid or expired OTP", http.StatusUnauthorized)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "otp_verified"})
}
