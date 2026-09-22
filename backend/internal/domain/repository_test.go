package domain

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func setupTestDB(t *testing.T) *pgxpool.Pool {
	// Aguarda o banco subir via docker-compose (tentativas)
	connStr := "postgres://logn_user:logn_password@localhost:5432/logn_db?sslmode=disable"
	var conn *pgxpool.Pool
	var err error

	for i := 0; i < 5; i++ {
		conn, err = pgxpool.New(context.Background(), connStr)
		if err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		t.Fatalf("Failed to connect to test db: %v", err)
	}

	// Limpa tabelas para testes isolados
	_, err = conn.Exec(context.Background(), "TRUNCATE TABLE game_events, user_sync_state, challenges CASCADE")
	if err != nil {
		t.Fatalf("Failed to truncate tables: %v", err)
	}

	return conn
}

func TestRepository_InsertChallenge(t *testing.T) {
	conn := setupTestDB(t)
	defer conn.Close()

	repo := NewRepository(conn)

	ctx := context.Background()

	// 1. Inserir Challenge válido
	validPayload := []byte(`{
		"content": {"code_lines": ["int a = 1;"]},
		"validation": {"type": "LINE_MATCH", "correct_line": 1}
	}`)
	// node_id é FK para skill_nodes desde que a árvore virou DAG: precisa do UUID
	// de um nó semeado, não de um rótulo solto.
	const adHocNode = "10000000-0000-0000-0000-000000000001"

	ch1 := Challenge{
		ID:           "test_bug_1",
		NodeID:      adHocNode,
		TemplateType: "SPOT_THE_BUG",
		Version:      1,
		Payload:      validPayload,
	}

	err := repo.InsertChallenge(ctx, ch1)
	if err != nil {
		t.Fatalf("Expected valid challenge insertion, got: %v", err)
	}

	// 2. Inserir Challenge inválido (sem code_lines no SPOT_THE_BUG)
	invalidPayload := []byte(`{
		"content": {"story": "missing code_lines"},
		"validation": {"type": "MATCH"}
	}`)
	ch2 := Challenge{
		ID:           "test_bug_2",
		NodeID:      adHocNode,
		TemplateType: "SPOT_THE_BUG",
		Version:      1,
		Payload:      invalidPayload,
	}

	err = repo.InsertChallenge(ctx, ch2)
	if err == nil || !strings.Contains(err.Error(), "chk_payload_structure") {
		t.Fatalf("Expected check constraint violation for missing code_lines, got: %v", err)
	}
}

func TestRepository_InsertSyncEvents(t *testing.T) {
	conn := setupTestDB(t)
	defer conn.Close()

	repo := NewRepository(conn)
	ctx := context.Background()

	userID := "user_123"

	// 1. Testa GetUserLastHash com usuário novo
	lastHash, err := repo.GetUserLastHash(ctx, userID)
	if err != nil {
		t.Fatalf("Expected no error for new user, got: %v", err)
	}
	if lastHash != "0000000000000000000000000000000000000000000000000000000000000000" {
		t.Fatalf("Expected genesis hash for new user, got: %s", lastHash)
	}

	// 2. Insere eventos de Sync
	event1 := GameEvent{
		ID:           "evt_1",
		EventType:    "SOLVE",
		PayloadJSON:  "{}",
		Timestamp:    1600000000,
		PreviousHash: lastHash,
	}
	event1.CurrentHash = ComputeHash(event1, event1.PreviousHash)

	payload := SyncPayload{
		UserID: userID,
		Events: []GameEvent{event1},
	}

	err = repo.InsertSyncEvents(ctx, payload, event1.CurrentHash)
	if err != nil {
		t.Fatalf("Expected successful sync insert, got: %v", err)
	}

	// 3. Testa se atualizou a last_hash do usuário
	newHash, err := repo.GetUserLastHash(ctx, userID)
	if err != nil || newHash != event1.CurrentHash {
		t.Fatalf("Expected updated hash %s, got %s (err: %v)", event1.CurrentHash, newHash, err)
	}
}
