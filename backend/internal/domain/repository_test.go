package domain

import (
	"context"
	"errors"
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

	// Limpa só o que os testes escrevem.
	//
	// Isto era um `TRUNCATE ... challenges CASCADE`: rodar a suíte apagava o seed do
	// banco de desenvolvimento, as migrações não repunham (já constavam aplicadas) e
	// a próxima partida abria sem problema nenhum.
	_, err = conn.Exec(context.Background(), "TRUNCATE TABLE game_events, user_sync_state CASCADE")
	if err != nil {
		t.Fatalf("Failed to truncate tables: %v", err)
	}
	if _, err = conn.Exec(context.Background(),
		"DELETE FROM challenges WHERE id LIKE 'test_%'"); err != nil {
		t.Fatalf("Failed to clear test challenges: %v", err)
	}

	return conn
}

func TestRepository_InsertChallenge(t *testing.T) {
	conn := setupTestDB(t)
	defer conn.Close()

	repo := NewRepository(conn)

	ctx := context.Background()

	// O teste não trunca mais a tabela — o seed precisa sobreviver a rodar a suíte —
	// então ele tem de recolher o que sujou. Sem isto, test_bug_1 ficava no banco e
	// aparecia como um problema a mais na partida do nó 1.
	defer func() {
		_, _ = conn.Exec(context.Background(),
			"DELETE FROM challenges WHERE id IN ('test_bug_1', 'test_bug_2')")
	}()

	// 1. Inserir Challenge válido
	// A explicação é exigida por constraint desde a 0005: desafio sem ela caía no texto
	// genérico, que é a regressão que a 0002 existiu para consertar.
	validPayload := []byte(`{
		"content": {"code_lines": ["int a = 1;"]},
		"validation": {"type": "LINE_MATCH", "correct_line": 1, "explanation": "A linha 1 é a única."}
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
		PositionIdx:  90, // fora da faixa dos seeds, para não colidir no UNIQUE do nó
	}

	err := repo.InsertChallenge(ctx, ch1)
	if err != nil {
		t.Fatalf("Expected valid challenge insertion, got: %v", err)
	}

	// 2. Inserir Challenge inválido (sem code_lines no SPOT_THE_BUG)
	// A explicação está aqui de propósito: sem ela a linha bateria em duas constraints
	// ao mesmo tempo, e a ordem em que o Postgres as avalia não é garantida — o teste
	// passaria a depender de sorte para ver chk_payload_structure na mensagem.
	invalidPayload := []byte(`{
		"content": {"story": "missing code_lines"},
		"validation": {"type": "MATCH", "correct_line": 1, "explanation": "Irrelevante."}
	}`)
	ch2 := Challenge{
		ID:           "test_bug_2",
		NodeID:      adHocNode,
		TemplateType: "SPOT_THE_BUG",
		Version:      1,
		Payload:      invalidPayload,
		PositionIdx:  91,
	}

	// Sem code_lines, um SPOT_THE_BUG viola duas constraints ao mesmo tempo:
	// chk_payload_structure, que exige a chave, e chk_spot_the_bug_linha_valida, que não
	// consegue conferir o intervalo de uma lista que não existe. O Postgres não promete
	// qual das duas ele reporta, então o teste aceita as duas — exigir uma era depender
	// de sorte.
	err = repo.InsertChallenge(ctx, ch2)
	if err == nil ||
		(!strings.Contains(err.Error(), "chk_payload_structure") &&
			!strings.Contains(err.Error(), "chk_spot_the_bug_linha_valida")) {
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

	err = repo.InsertSyncEvents(ctx, payload, lastHash, event1.CurrentHash)
	if err != nil {
		t.Fatalf("Expected successful sync insert, got: %v", err)
	}

	// 3. Testa se atualizou a last_hash do usuário
	newHash, err := repo.GetUserLastHash(ctx, userID)
	if err != nil || newHash != event1.CurrentHash {
		t.Fatalf("Expected updated hash %s, got %s (err: %v)", event1.CurrentHash, newHash, err)
	}

	// 4. Um segundo sync que validou contra o topo antigo não grava. É o que acontece
	// com dois syncs em paralelo: o segundo contaria o XP de novo.
	stale := GameEvent{
		ID:           "evt_stale",
		EventType:    "SOLVE",
		PayloadJSON:  "{}",
		Timestamp:    1600000001,
		PreviousHash: lastHash,
	}
	stale.CurrentHash = ComputeHash(stale, stale.PreviousHash)
	err = repo.InsertSyncEvents(ctx, SyncPayload{UserID: userID, Events: []GameEvent{stale}}, lastHash, stale.CurrentHash)
	if !errors.Is(err, ErrStaleChain) {
		t.Fatalf("esperava ErrStaleChain para topo antigo, veio: %v", err)
	}
	if top, _ := repo.GetUserLastHash(ctx, userID); top != event1.CurrentHash {
		t.Fatalf("o topo não podia ter andado: %s", top)
	}

	// 5. O mesmo id de evento em outro usuário não colide. O cliente gera ids a
	// partir do timestamp, e com a chave global um jogador travava o sync do outro.
	other := "user_456"
	otherGenesis, _ := repo.GetUserLastHash(ctx, other)
	twin := event1
	twin.CurrentHash = ComputeHash(twin, otherGenesis)
	err = repo.InsertSyncEvents(ctx, SyncPayload{UserID: other, Events: []GameEvent{twin}}, otherGenesis, twin.CurrentHash)
	if err != nil {
		t.Fatalf("mesmo id em outro usuário devia gravar, veio: %v", err)
	}
}
