package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/josecleiton/logn/backend/internal/domain"
)

func setupTestDB(t *testing.T) *pgxpool.Pool {
	connStr := "postgres://logn:lognpassword@localhost:5432/logndb?sslmode=disable"
	var conn *pgxpool.Pool
	var err error

	for i := 0; i < 5; i++ {
		conn, err = pgxpool.New(context.Background(), connStr)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if err != nil {
		t.Fatalf("Failed to connect to test db: %v", err)
	}

	_, err = conn.Exec(context.Background(), "TRUNCATE TABLE game_events, user_sync_state, challenges CASCADE")
	if err != nil {
		t.Fatalf("Failed to truncate tables: %v", err)
	}

	return conn
}

func TestSyncHandler(t *testing.T) {
	conn := setupTestDB(t)
	defer conn.Close()

	repo := domain.NewRepository(conn)
	server := &Server{repo: repo}

	// 1. Valid Sync Post
	event1 := domain.GameEvent{
		ID:           "api_evt_1",
		EventType:    "SOLVE",
		PayloadJSON:  "{}",
		Timestamp:    1600000000,
		PreviousHash: "0000000000000000000000000000000000000000000000000000000000000000",
	}
	event1.CurrentHash = domain.ComputeHash(event1, event1.PreviousHash)

	payload := domain.SyncPayload{
		UserID: "user_api",
		Events: []domain.GameEvent{event1},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/sync", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	
	rr := httptest.NewRecorder()
	server.syncHandler(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	if response["status"] != "success" {
		t.Errorf("Expected success, got %v", response["status"])
	}

	// 2. Conflict Sync Post (Rebase required)
	event2 := domain.GameEvent{
		ID:           "api_evt_2",
		EventType:    "SOLVE",
		PayloadJSON:  "{}",
		Timestamp:    1600000001,
		PreviousHash: "0000000000000000000000000000000000000000000000000000000000000000", // Wrong previous hash
	}
	event2.CurrentHash = domain.ComputeHash(event2, event2.PreviousHash)

	payloadConflict := domain.SyncPayload{
		UserID: "user_api",
		Events: []domain.GameEvent{event2},
	}

	bodyConflict, _ := json.Marshal(payloadConflict)
	reqConflict := httptest.NewRequest("POST", "/api/v1/sync", bytes.NewReader(bodyConflict))
	
	rrConflict := httptest.NewRecorder()
	server.syncHandler(rrConflict, reqConflict)

	if status := rrConflict.Code; status != http.StatusConflict {
		t.Errorf("handler returned wrong status code for conflict: got %v want %v", status, http.StatusConflict)
	}
}
