package domain

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// Rejogar um nó não paga de novo.
//
// Cada resposta aceita somava 50, e dava para abrir o nó 7 repetindo o nó 1. O servidor
// é quem manda no XP — o snapshot dele sobrescreve o do aparelho no login —, então a
// regra tem de valer aqui, não só na tela.
func TestProcessEventXPPagaUmaVezPorDesafio(t *testing.T) {
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

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range events {
		if err := repo.ProcessEventXP(ctx, tx, userID, e); err != nil {
			tx.Rollback(ctx)
			t.Fatalf("processando %s: %v", e.PayloadJSON, err)
		}
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
	if err := repo.ProcessEventXP(ctx, replay, userID, answer("ch_xp_1", true, "SPOT_THE_BUG")); err != nil {
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
