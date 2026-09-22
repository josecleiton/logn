package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
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
			log.Printf("sync rebase: user=%s eventos=%d topo_servidor=%s primeiro_previous=%s",
				payload.UserID, len(payload.Events), serverLastHash, payload.Events[0].PreviousHash)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":         "rebase_required",
				"server_top":     serverLastHash,
				"events_applied": 0,
			})
			return
		}

		// Sem este log, um sync recusado some: o cliente só vê o número do status e
		// o servidor não conta o motivo a ninguém.
		log.Printf("sync recusado: user=%s eventos=%d motivo=%v", payload.UserID, len(payload.Events), err)
		http.Error(w, "Security validation failed: "+err.Error(), http.StatusForbidden)
		return
	}

	if !valid {
		log.Printf("sync recusado: user=%s cadeia inválida", payload.UserID)
		http.Error(w, "Invalid chain", http.StatusForbidden)
		return
	}

	// Topo da cadeia depois deste sync. Devolver o topo antigo faria o cliente
	// continuar encadeando a partir de onde o servidor já não está.
	newTop := serverLastHash
	if len(payload.Events) > 0 {
		newTop = payload.Events[len(payload.Events)-1].CurrentHash
		if err := s.repo.InsertSyncEvents(ctx, payload, newTop); err != nil {
			log.Printf("sync não gravado: user=%s eventos=%d erro=%v", payload.UserID, len(payload.Events), err)
			http.Error(w, "Failed to save events: "+err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("sync ok: user=%s eventos=%d topo=%s", payload.UserID, len(payload.Events), newTop)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "success",
		"events_applied": len(payload.Events),
		"new_top":        newTop,
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
		dbUrl = "postgres://logn_user:logn_password@localhost:5432/logn_db?sslmode=disable"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Println("WARNING: JWT_SECRET is not set. Using insecure default for development.")
		domain.JwtSecretKey = []byte("my-super-secret-logn-key-for-dev")
	} else {
		domain.JwtSecretKey = []byte(jwtSecret)
	}

	pool, err := pgxpool.New(context.Background(), dbUrl)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("Unable to reach database: %v\n", err)
	}

	repo := domain.NewRepository(pool)
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
