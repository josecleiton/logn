package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
	"slices"

	"github.com/josecleiton/logn/backend/internal/domain"
	"github.com/josecleiton/logn/backend/internal/legal"
)

// pendingLegalHandler lista as versões vigentes que a conta ainda não aceitou.
//
// Lia o dono de `r.Context().Value("user_id")`, que nada preenche: a asserção de tipo
// derrubava toda requisição. O dono sai do token, como no resto da API.
func (s *Server) pendingLegalHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.authenticate(w, r)
	if !ok {
		return
	}

	docs, err := s.repo.GetPendingLegalDocuments(r.Context(), userID)
	if err != nil {
		log.Printf("pendências legais não lidas: user=%s erro=%v", userID, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
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
	userID, ok := s.authenticate(w, r)
	if !ok {
		return
	}

	var req AcceptLegalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}

	current, err := s.currentLegalVersions(r.Context())
	if err != nil {
		log.Printf("aceite sem versões legais: user=%s erro=%v", userID, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}
	doc, ok := current[legal.Kind(req.Kind)]
	if !ok || !slices.Contains(legal.Locales, req.Locale) {
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	// Só a versão vigente se aceita. Aceitar uma velha não resolve pendência nenhuma,
	// e aceitar uma que não existe gravaria lixo.
	if req.Version != doc.Version {
		writeError(w, http.StatusConflict, codeLegalVersionOutdated)
		return
	}

	if err := s.repo.AcceptLegalDocument(r.Context(), userID, req.Kind, req.Version, req.Locale); err != nil {
		log.Printf("aceite não gravado: user=%s erro=%v", userID, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}

	w.WriteHeader(http.StatusOK)
}
