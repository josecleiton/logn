package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/josecleiton/logn/backend/internal/domain"
	"github.com/josecleiton/logn/backend/internal/infrastructure/email"
)

type Server struct {
	repo   *domain.Repository
	mailer *email.Mailer
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

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Println("WARNING: JWT_SECRET is not set. Using insecure default for development.")
		domain.JwtSecretKey = []byte("my-super-secret-logn-key-for-dev")
	} else {
		domain.JwtSecretKey = []byte(jwtSecret)
	}

	conn, err := pgx.Connect(context.Background(), dbUrl)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer conn.Close(context.Background())

	repo := domain.NewRepository(conn)
	mailer := email.NewMailer()
	server := &Server{repo: repo, mailer: mailer}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", server.pingHandler)
	mux.HandleFunc("POST /api/v1/sync", server.syncHandler)
	mux.HandleFunc("GET /api/v1/challenges", server.challengesHandler)
	mux.HandleFunc("POST /api/v1/auth/login", server.loginHandler)
	mux.HandleFunc("POST /api/v1/auth/refresh", server.refreshHandler)
	mux.HandleFunc("POST /api/v1/auth/request-otp", server.requestOTPHandler)
	mux.HandleFunc("POST /api/v1/auth/verify-otp", server.verifyOTPHandler)
	mux.HandleFunc("POST /api/v1/auth/register", server.registerHandler)
	mux.HandleFunc("POST /api/v1/auth/reset-password", server.resetPasswordHandler)


	mux.HandleFunc("GET /api/v1/nodes", server.getNodesHandler)
	mux.HandleFunc("GET /api/v1/progress", server.getUserProgressHandler)

	log.Println("Server starting on :8080...")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
