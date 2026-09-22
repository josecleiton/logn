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
	connStr := "postgres://logn_user:logn_password@localhost:5432/logn_db?sslmode=disable"
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

	// Os desafios ficam: a suíte não tem nada a ver com eles, e apagá-los deixava o
	// banco de desenvolvimento sem seed até alguém reparar que a partida abre vazia.
	_, err = conn.Exec(context.Background(), "TRUNCATE TABLE game_events, user_sync_state CASCADE")
	if err != nil {
		t.Fatalf("Failed to truncate tables: %v", err)
	}

	return conn
}

// syncUserID é um usuário de verdade: `game_events.user_id` é UUID e o handler tira
// o dono do token, não do corpo do pedido.
const syncUserID = "11111111-2222-3333-4444-555555555555"

// bearer devolve um `Authorization` válido para o usuário do teste.
func bearer(t *testing.T, userID string) string {
	t.Helper()
	domain.JwtSecretKey = []byte("test-secret")
	token, err := domain.GenerateAccessToken(userID)
	if err != nil {
		t.Fatalf("Failed to mint token: %v", err)
	}
	return "Bearer " + token
}

func TestSyncHandler(t *testing.T) {
	conn := setupTestDB(t)
	defer conn.Close()

	repo := domain.NewRepository(conn)
	server := &Server{repo: repo}
	auth := bearer(t, syncUserID)

	// 0. Sem token não passa. O corpo do pedido dizia de quem era a cadeia e o
	// servidor obedecia: dava para escrever eventos na conta de qualquer um.
	reqAnon := httptest.NewRequest("POST", "/api/v1/sync", bytes.NewReader([]byte(`{"user_id":"x","events":[]}`)))
	rrAnon := httptest.NewRecorder()
	server.syncHandler(rrAnon, reqAnon)
	if rrAnon.Code != http.StatusUnauthorized {
		t.Fatalf("sync sem token: got %v want %v", rrAnon.Code, http.StatusUnauthorized)
	}

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
		// De propósito diferente do dono do token: o handler tem de ignorar isto.
		UserID: "00000000-0000-0000-0000-00000000dead",
		Events: []domain.GameEvent{event1},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/sync", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", auth)

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

	// O evento foi para a cadeia do token, não para a do corpo.
	storedHash, err := repo.GetUserLastHash(context.Background(), syncUserID)
	if err != nil || storedHash != event1.CurrentHash {
		t.Fatalf("esperava a cadeia do token em %s, topo %s (err: %v)", syncUserID, storedHash, err)
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
		UserID: syncUserID,
		Events: []domain.GameEvent{event2},
	}

	bodyConflict, _ := json.Marshal(payloadConflict)
	reqConflict := httptest.NewRequest("POST", "/api/v1/sync", bytes.NewReader(bodyConflict))
	reqConflict.Header.Set("Authorization", auth)

	rrConflict := httptest.NewRecorder()
	server.syncHandler(rrConflict, reqConflict)

	if status := rrConflict.Code; status != http.StatusConflict {
		t.Errorf("handler returned wrong status code for conflict: got %v want %v", status, http.StatusConflict)
	}
}
