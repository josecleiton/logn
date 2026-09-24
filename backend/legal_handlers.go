package main

import (
	"encoding/json"
	"net/http"

	"github.com/josecleiton/logn/backend/internal/domain"
)

func (s *Server) pendingLegalHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(string)

	docs, err := s.repo.GetPendingLegalDocuments(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if docs == nil {
		docs = []domain.LegalDocument{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(docs)
}

type AcceptLegalRequest struct {
	Kind    string `json:"kind"`
	Version int    `json:"version"`
	Locale  string `json:"locale"`
}

func (s *Server) acceptLegalHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(string)

	var req AcceptLegalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := s.repo.AcceptLegalDocument(r.Context(), userID, req.Kind, req.Version, req.Locale)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
