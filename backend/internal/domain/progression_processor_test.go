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

	email := fmt.Sprintf("xp_%d@test.logn", time.Now().UnixNano())
	var userID string
	if err := conn.QueryRow(ctx, `INSERT INTO users (email) VALUES ($1) RETURNING id`, email).Scan(&userID); err != nil {
		t.Fatalf("criando usuário: %v", err)
	}
	defer conn.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)

	const adHocNode = "10000000-0000-0000-0000-000000000001"
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
}
