package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

	// Nada de TRUNCATE: este é o banco de desenvolvimento, e os pacotes rodam em
	// paralelo. Cada teste escreve com um usuário só dele e apaga só o que escreveu.
	return conn
}

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

// O conteúdo da trilha diz em que língua saiu e avisa o cache de que muda com ela. Pelo
// caminho de produção, com o gzip na frente: ele também mexe em Vary, e um Set no
// lugar de Add apagaria o outro.
func TestContentResponsesCarryTheirLanguage(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	server := &Server{repo: domain.NewRepository(conn)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/challenges", server.challengesHandler)
	mux.HandleFunc("GET /api/v1/nodes", server.getNodesHandler)
	handler := withGzip(mux)

	for _, path := range []string{"/api/v1/challenges", "/api/v1/nodes"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Accept-Language", "es-MX,es;q=0.9")
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d", path, rec.Code)
		}
		if got := rec.Header().Get("Content-Language"); got != "es" {
			t.Errorf("%s: Content-Language %q, esperava es", path, got)
		}
		vary := strings.Join(rec.Header().Values("Vary"), ",")
		for _, h := range []string{"Accept-Language", "Accept-Encoding"} {
			if !strings.Contains(vary, h) {
				t.Errorf("%s: Vary %q sem %s", path, vary, h)
			}
		}
		if got := rec.Header().Get("Content-Type"); got != "application/json" {
			t.Errorf("%s: Content-Type %q", path, got)
		}

		// HEAD responde como GET, sem corpo. Uma guarda de GET no handler dava 405.
		head := httptest.NewRequest(http.MethodHead, path, nil)
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, head)
		if rec.Code != http.StatusOK {
			t.Errorf("HEAD %s: status %d", path, rec.Code)
		}
		if got := rec.Header().Get("Content-Language"); got != "pt-BR" {
			t.Errorf("HEAD %s: Content-Language %q", path, got)
		}
	}
}

func TestSyncHandler(t *testing.T) {
	conn := setupTestDB(t)
	// Cleanup, e não defer: o defer fecharia o pool antes da limpeza abaixo rodar.
	t.Cleanup(conn.Close)

	repo := domain.NewRepository(conn)
	server := &Server{repo: repo}
	// Um usuário de verdade e só deste teste: o handler tira o dono do token, e uma
	// cadeia compartilhada com outro teste (ou outra execução) mudaria o topo dela.
	syncUserID := newTestUUID(t)
	auth := bearer(t, syncUserID)

	// O token só vale para conta que existe e não pediu exclusão (ac84a44): sem a linha
	// em users, o sync respondia 401 e o teste falhava desde então.
	ctx := context.Background()
	if _, err := conn.Exec(ctx,
		`INSERT INTO users (id, email, anon_number) VALUES ($1, $2, $3)`,
		syncUserID, "sync-"+syncUserID+"@example.com", testAnonNumber(t)); err != nil {
		t.Fatalf("criando o usuário do teste: %v", err)
	}
	t.Cleanup(func() {
		conn.Exec(ctx, `DELETE FROM game_events WHERE user_id = $1`, syncUserID)
		conn.Exec(ctx, `DELETE FROM user_sync_state WHERE user_id = $1`, syncUserID)
		conn.Exec(ctx, `DELETE FROM users WHERE id = $1`, syncUserID)
	})

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
		EventType:    "MATCH_ANSWER",
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
		EventType:    "MATCH_ANSWER",
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
