package domain

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// O lote paga o mesmo que o laço evento a evento: mesmos números em `users`, mesmo
// progresso e desbloqueio por nó, mesmos desafios pagos. Uma conta recebe os eventos um
// por sync, a outra todos num sync só.
func TestProcessEventsXPInABatchMatchesOneByOne(t *testing.T) {
	conn := setupTestDB(t)
	defer conn.Close()
	repo := NewRepository(conn)
	ctx := context.Background()

	const nodeA, nodeB = "90000000-0000-0000-0000-0000000000b1", "90000000-0000-0000-0000-0000000000b2"
	cleanup := func() {
		conn.Exec(ctx, `DELETE FROM challenges WHERE id LIKE 'test_batch_%'`)
		conn.Exec(ctx, `DELETE FROM skill_nodes WHERE id IN ($1, $2)`, nodeA, nodeB)
	}
	cleanup()
	defer cleanup()
	// A abre com dois acertos; B pede três e recebe dois.
	if _, err := conn.Exec(ctx, `INSERT INTO skill_nodes (id, track_id, row_idx, col_idx, required_xp, prerequisites) VALUES
		($1, '00000000-0000-0000-0000-000000000000', 96, 0, 100, '[]'),
		($2, '00000000-0000-0000-0000-000000000000', 97, 0, 150, '[]')`, nodeA, nodeB); err != nil {
		t.Fatalf("nós: %v", err)
	}
	const code = `{"content":{"code_lines":["int a;"]},"validation":{"type":"LINE_MATCH","correct_line":1}}`
	if _, err := conn.Exec(ctx, `INSERT INTO challenges (id, node_id, template_type, payload, position_idx) VALUES
		('test_batch_1', $1, 'SPOT_THE_BUG', $3, 1), ('test_batch_2', $1, 'SPOT_THE_BUG', $3, 2),
		('test_batch_3', $2, 'SPOT_THE_BUG', $3, 1), ('test_batch_4', $2, 'SPOT_THE_BUG', $3, 2)`,
		nodeA, nodeB, code); err != nil {
		t.Fatalf("desafios: %v", err)
	}

	answer := func(challengeID string, correct bool, template string) GameEvent {
		return GameEvent{EventType: "MATCH_ANSWER", PayloadJSON: fmt.Sprintf(
			`{"is_correct":%t,"template_type":"%s","challenge_id":"%s"}`, correct, template, challengeID)}
	}
	events := []GameEvent{
		answer("test_batch_1", true, "SPOT_THE_BUG"),
		answer("test_batch_2", false, "DRY_RUN"),
		answer("test_batch_3", true, "DRY_RUN"),
		answer("test_batch_1", true, "SPOT_THE_BUG"), // rejogo
		answer("test_batch_2", true, "DRY_RUN"),      // acerto depois do erro: abre A
		answer("test_batch_invented", true, "DRY_RUN"),
		{EventType: "MATCH_ANSWER", PayloadJSON: `{"is_correct":`}, // não decodifica
		{EventType: "MATCH_END", PayloadJSON: `{}`},
		answer("test_batch_4", true, "SPOT_THE_BUG"),
	}

	newUser := func() string {
		var id string
		email := fmt.Sprintf("batch_%d@example.com", time.Now().UnixNano())
		if err := conn.QueryRow(ctx, `INSERT INTO users (email, anon_number) VALUES ($1, $2) RETURNING id`, email, farAnonNumber(t)).Scan(&id); err != nil {
			t.Fatalf("criando usuário: %v", err)
		}
		t.Cleanup(func() { conn.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id) })
		return id
	}
	process := func(userID string, events []GameEvent) {
		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if err := repo.ProcessEventsXP(ctx, tx, userID, events); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	snapshot := func(userID string) string {
		var s string
		if err := conn.QueryRow(ctx, `
			SELECT format('%s/%s/%s/%s/%s|%s|%s',
			       u.global_xp, u.bugs_found, u.dry_runs_completed, u.free_xp, u.free_xp_reached_at IS NOT NULL,
			       (SELECT string_agg(format('%s:%s:%s:%s', node_id, current_xp, unlocked, completed_at IS NOT NULL), ',' ORDER BY node_id)
			        FROM user_progress WHERE user_id = u.id),
			       (SELECT string_agg(challenge_id, ',' ORDER BY challenge_id) FROM user_paid_challenges WHERE user_id = u.id))
			FROM users u WHERE u.id = $1`, userID).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}

	oneByOne, batched := newUser(), newUser()
	for _, e := range events {
		process(oneByOne, []GameEvent{e})
	}
	process(batched, events)

	want := fmt.Sprintf("%d/2/2/%d/t|%s:%d:t:t,%s:%d:f:f|test_batch_1,test_batch_2,test_batch_3,test_batch_4",
		4*XPPerAcceptedAnswer, 4*XPPerAcceptedAnswer, nodeA, 2*XPPerAcceptedAnswer, nodeB, 2*XPPerAcceptedAnswer)
	if got := snapshot(oneByOne); got != want {
		t.Fatalf("evento a evento:\n got %s\nwant %s", got, want)
	}
	if got := snapshot(batched); got != want {
		t.Fatalf("em lote:\n got %s\nwant %s", got, want)
	}

	// A migração 0071 reconstrói o desempate do placar pela hora das linhas pagas: no
	// lote, a hora do XP gratuito tem de ser a mesma delas.
	var same bool
	if err := conn.QueryRow(ctx, `
		SELECT u.free_xp_reached_at = (SELECT max(created_at) FROM user_paid_challenges WHERE user_id = u.id)
		FROM users u WHERE u.id = $1`, batched).Scan(&same); err != nil {
		t.Fatal(err)
	}
	if !same {
		t.Error("free_xp_reached_at diferente da hora das linhas pagas")
	}
}

// Rejogar um nó não paga de novo.
//
// Cada resposta aceita somava 50, e dava para abrir o nó 7 repetindo o nó 1. O servidor
// é quem manda no XP — o snapshot dele sobrescreve o do aparelho no login —, então a
// regra tem de valer aqui, não só na tela.
func TestProcessEventsXPPagaUmaVezPorDesafio(t *testing.T) {
	conn := setupTestDB(t)
	defer conn.Close()
	repo := NewRepository(conn)
	ctx := context.Background()

	email := fmt.Sprintf("xp_%d@example.com", time.Now().UnixNano())
	var userID string
	if err := conn.QueryRow(ctx, `INSERT INTO users (email, anon_number) VALUES ($1, $2) RETURNING id`, email, farAnonNumber(t)).Scan(&userID); err != nil {
		t.Fatalf("criando usuário: %v", err)
	}
	defer conn.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)

	// Os desafios existem no banco, num nó da trilha gratuita: id que o banco não
	// conhece não paga mais.
	const adHocNode = "90000000-0000-0000-0000-0000000000a1"
	cleanup := func() {
		conn.Exec(ctx, `DELETE FROM challenges WHERE id IN ('ch_xp_1', 'ch_xp_2')`)
		conn.Exec(ctx, `DELETE FROM skill_nodes WHERE id = $1`, adHocNode)
	}
	cleanup()
	defer cleanup()
	if _, err := conn.Exec(ctx, `INSERT INTO skill_nodes (id, track_id, row_idx, col_idx, required_xp, prerequisites)
		VALUES ($1, '00000000-0000-0000-0000-000000000000', 95, 0, 0, '[]')`, adHocNode); err != nil {
		t.Fatalf("nó: %v", err)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO challenges (id, node_id, template_type, payload, position_idx) VALUES
		('ch_xp_1', $1, 'SPOT_THE_BUG', '{"content":{"code_lines":["int a;"]},"validation":{"type":"LINE_MATCH","correct_line":1}}', 1),
		('ch_xp_2', $1, 'SPOT_THE_BUG', '{"content":{"code_lines":["int b;"]},"validation":{"type":"LINE_MATCH","correct_line":1}}', 2)`, adHocNode); err != nil {
		t.Fatalf("desafios: %v", err)
	}

	answer := func(challengeID string, correct bool, template string) GameEvent {
		return GameEvent{
			EventType: "MATCH_ANSWER",
			PayloadJSON: fmt.Sprintf(`{"letter":"A","is_correct":%t,"template_type":"%s","node_id":"%s","challenge_id":"%s"}`,
				correct, template, adHocNode, challengeID),
		}
	}

	events := []GameEvent{
		answer("ch_xp_1", true, "SPOT_THE_BUG"),
		answer("ch_xp_1", true, "SPOT_THE_BUG"), // rejogou: não paga
		answer("ch_xp_2", false, "DRY_RUN"),     // errou: não paga nem marca
		answer("ch_xp_2", true, "DRY_RUN"),      // acertou depois: paga
		answer("", true, "SPOT_THE_BUG"),        // app antigo, sem id: não paga
	}

	// O lote inteiro de uma vez, como o sync manda: o rejogo e o erro seguido de acerto
	// estão no mesmo lote, e o resultado tem de ser o do laço evento a evento.
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.ProcessEventsXP(ctx, tx, userID, events); err != nil {
		tx.Rollback(ctx)
		t.Fatalf("processando o lote: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	stats, err := repo.GetUserStats(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if stats.GlobalXP != 2*XPPerAcceptedAnswer {
		t.Errorf("global_xp = %d, esperava %d", stats.GlobalXP, 2*XPPerAcceptedAnswer)
	}
	if stats.BugsFound != 1 || stats.DryRunsCompleted != 1 {
		t.Errorf("bugs=%d dry_runs=%d, esperava 1 e 1", stats.BugsFound, stats.DryRunsCompleted)
	}
	if len(stats.PaidChallengeIDs) != 2 || stats.PaidChallengeIDs[0] != "ch_xp_1" || stats.PaidChallengeIDs[1] != "ch_xp_2" {
		t.Errorf("paid_challenge_ids = %v, esperava [ch_xp_1 ch_xp_2]", stats.PaidChallengeIDs)
	}
	if len(stats.Nodes) != 1 || stats.Nodes[0].CurrentXP != 2*XPPerAcceptedAnswer {
		t.Errorf("progresso do nó = %+v, esperava um nó com %d", stats.Nodes, 2*XPPerAcceptedAnswer)
	}

	// O nó é da trilha gratuita: o placar anda junto, e o rejogo não anda nada.
	freeXP, reached := freeXPOf(t, repo, userID)
	if freeXP != 2*XPPerAcceptedAnswer || !reached {
		t.Errorf("free_xp = %d (reached %v), esperava %d com hora", freeXP, reached, 2*XPPerAcceptedAnswer)
	}
	// A mesma conta que as migrações 0070 e 0071 fazem.
	var counted int
	if err := conn.QueryRow(ctx, `
		SELECT 50 * count(*)
		FROM user_paid_challenges up
		JOIN challenges c ON c.id = up.challenge_id
		JOIN skill_nodes sn ON sn.id = c.node_id
		JOIN tracks t ON t.id = sn.track_id
		WHERE up.user_id = $1 AND t.kind = 'free'`, userID).Scan(&counted); err != nil {
		t.Fatal(err)
	}
	if counted != freeXP {
		t.Errorf("a conta da migração dá %d e o processamento deu %d", counted, freeXP)
	}

	replay, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.ProcessEventsXP(ctx, replay, userID, []GameEvent{answer("ch_xp_1", true, "SPOT_THE_BUG")}); err != nil {
		replay.Rollback(ctx)
		t.Fatal(err)
	}
	if err := replay.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if again, _ := freeXPOf(t, repo, userID); again != freeXP {
		t.Errorf("o replay de MATCH_ANSWER andou o placar: %d -> %d", freeXP, again)
	}
}
