package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/josecleiton/logn/backend/internal/domain"
)

func pingHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "LogN Backend is running!",
	})
}

func syncHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload domain.SyncPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Mockando o hash do servidor por enquanto
	serverLastHash := "0000000000000000000000000000000000000000000000000000000000000000"

	valid, err := domain.ValidateSync(payload, serverLastHash)
	if err != nil {
		if err.Error() == "force_rebase" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict) // 409 Conflict trigger rebase
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "success",
		"events_applied": len(payload.Events),
		"new_top":        payload.Events[len(payload.Events)-1].CurrentHash,
	})
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", pingHandler)
	mux.HandleFunc("POST /api/v1/sync", syncHandler)

	log.Println("Server starting on :8080...")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
