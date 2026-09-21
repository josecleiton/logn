package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/josecleiton/logn/backend/internal/domain"
)

type Server struct {
	repo *domain.Repository
}

func (s *Server) pingHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "LogN Backend is running!",
	})
}

func (s *Server) syncHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload domain.SyncPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	serverLastHash, err := s.repo.GetUserLastHash(ctx, payload.UserID)
	if err != nil {
		http.Error(w, "Failed to get user state: "+err.Error(), http.StatusInternalServerError)
		return
	}

	valid, err := domain.ValidateSync(payload, serverLastHash)
	if err != nil {
		if err.Error() == "force_rebase" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":         "rebase_required",
				"server_top":     serverLastHash,
				"events_applied": 0,
			})
			return
		}

		http.Error(w, "Security validation failed: "+err.Error(), http.StatusForbidden)
		return
	}

	if !valid {
		http.Error(w, "Invalid chain", http.StatusForbidden)
		return
	}

	if len(payload.Events) > 0 {
		newTop := payload.Events[len(payload.Events)-1].CurrentHash
		if err := s.repo.InsertSyncEvents(ctx, payload, newTop); err != nil {
			http.Error(w, "Failed to save events: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "success",
		"events_applied": len(payload.Events),
		"new_top":        serverLastHash,
	})
}

func (s *Server) challengesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := context.Background()
	challenges, err := s.repo.GetChallenges(ctx)
	if err != nil {
		http.Error(w, "Failed to get challenges: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(challenges)
}

func main() {
	dbUrl := os.Getenv("DATABASE_URL")
	if dbUrl == "" {
		dbUrl = "postgres://logn:lognpassword@localhost:5432/logndb?sslmode=disable"
	}

	conn, err := pgx.Connect(context.Background(), dbUrl)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer conn.Close(context.Background())

	repo := domain.NewRepository(conn)
	server := &Server{repo: repo}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", server.pingHandler)
	mux.HandleFunc("POST /api/v1/sync", server.syncHandler)
	mux.HandleFunc("GET /api/v1/challenges", server.challengesHandler)
	mux.HandleFunc("POST /api/v1/auth/login", server.loginHandler)
	mux.HandleFunc("POST /api/v1/auth/refresh", server.refreshHandler)

	log.Println("Server starting on :8080...")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
