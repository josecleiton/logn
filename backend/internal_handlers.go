package main

import (
	"net/http"
	"os"
)

func (s *Server) purgeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	authHeader := r.Header.Get("Authorization")
	expectedAudience := os.Getenv("CLOUD_SCHEDULER_AUDIENCE")
	if expectedAudience == "" {
		// Default to Cloud Run URL or some valid identifier if not explicitly set
		http.Error(w, "Endpoint disabled, missing audience config", http.StatusForbidden)
		return
	}

	if err := s.cloudValidator.ValidateToken(r.Context(), authHeader, expectedAudience); err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := s.repo.PurgeDeletedAccounts(r.Context()); err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}
