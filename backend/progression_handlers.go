package main

import (
	"encoding/json"
	"log"
	"net/http"
)

func (s *Server) getNodesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	nodes, err := s.repo.GetSkillNodes(r.Context())
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nodes)
}

func (s *Server) getUserProgressHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Era um UUID fixo, escrito à mão, que não pertencia a ninguém: a rota respondia
	// `null` para todo mundo e nenhum cliente a chamava.
	userID, ok := authenticate(w, r)
	if !ok {
		return
	}

	stats, err := s.repo.GetUserStats(r.Context(), userID)
	if err != nil {
		log.Printf("progresso não lido: user=%s erro=%v", userID, err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
