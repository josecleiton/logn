package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
)

type EventPayload struct {
	IsCorrect    bool   `json:"is_correct"`
	ChallengeID  string `json:"challenge_id"`
	NodeID       string `json:"node_id"`
	TemplateType string `json:"template_type"`
}

// XPPerAcceptedAnswer é a mesma regra que o Core aplica na tela: 50 por balão.
const XPPerAcceptedAnswer = 50

func (r *Repository) ProcessEventXP(ctx context.Context, tx pgx.Tx, userID string, event GameEvent) error {
	if event.EventType == "CHALLENGE_ANSWERED" || event.EventType == "challenge_answered" {
		var payload EventPayload
		if err := json.Unmarshal([]byte(event.PayloadJSON), &payload); err != nil {
			return nil
		}
		if !payload.IsCorrect || payload.NodeID == "" {
			return nil
		}
		
		xpEarned := 50

		upsertQuery := `
			INSERT INTO user_progress (user_id, node_id, current_xp, unlocked)
			VALUES ($1, $2, $3, false)
			ON CONFLICT (user_id, node_id) DO UPDATE 
			SET current_xp = user_progress.current_xp + EXCLUDED.current_xp`

		if _, err := tx.Exec(ctx, upsertQuery, userID, payload.NodeID, xpEarned); err != nil {
			return fmt.Errorf("failed to upsert progress: %w", err)
		}

		checkQuery := `
			UPDATE user_progress up
			SET unlocked = true, completed_at = CURRENT_TIMESTAMP
			FROM skill_nodes sn
			WHERE up.node_id = sn.id 
			  AND up.user_id = $1 
			  AND up.node_id = $2 
			  AND up.current_xp >= sn.required_xp
			  AND up.unlocked = false`

		if _, err := tx.Exec(ctx, checkQuery, userID, payload.NodeID); err != nil {
			return fmt.Errorf("failed to unlock node: %w", err)
		}
	} else if event.EventType == "MATCH_ANSWER" {
		var payload EventPayload
		if err := json.Unmarshal([]byte(event.PayloadJSON), &payload); err != nil {
			return nil
		}
		if !payload.IsCorrect {
			return nil
		}

		bugsEarned := 0
		if payload.TemplateType == "SPOT_THE_BUG" {
			bugsEarned = 1
		}
		// Dry run é o traçado de código aceito, não o fim da partida. Estava somando
		// um a cada MATCH_END: quem jogasse sem tocar num DRY_RUN ganhava o número.
		dryRunsEarned := 0
		if payload.TemplateType == "DRY_RUN" {
			dryRunsEarned = 1
		}

		updateUser := `
			UPDATE users
			SET global_xp = global_xp + $1,
			    bugs_found = bugs_found + $2,
			    dry_runs_completed = dry_runs_completed + $3
			WHERE id = $4`
		if _, err := tx.Exec(ctx, updateUser, XPPerAcceptedAnswer, bugsEarned, dryRunsEarned, userID); err != nil {
			return fmt.Errorf("failed to update user stats: %w", err)
		}

		// Progresso por nó. O evento só passou a carregar `node_id` agora; sem ele a
		// tabela `user_progress` ficava vazia mesmo com o jogador subindo balões.
		if payload.NodeID == "" {
			return nil
		}

		upsertQuery := `
			INSERT INTO user_progress (user_id, node_id, current_xp, unlocked)
			VALUES ($1, $2, $3, false)
			ON CONFLICT (user_id, node_id) DO UPDATE
			SET current_xp = user_progress.current_xp + EXCLUDED.current_xp`
		if _, err := tx.Exec(ctx, upsertQuery, userID, payload.NodeID, XPPerAcceptedAnswer); err != nil {
			return fmt.Errorf("failed to upsert progress: %w", err)
		}

		checkQuery := `
			UPDATE user_progress up
			SET unlocked = true, completed_at = CURRENT_TIMESTAMP
			FROM skill_nodes sn
			WHERE up.node_id = sn.id
			  AND up.user_id = $1
			  AND up.node_id = $2
			  AND up.current_xp >= sn.required_xp
			  AND up.unlocked = false`
		if _, err := tx.Exec(ctx, checkQuery, userID, payload.NodeID); err != nil {
			return fmt.Errorf("failed to unlock node: %w", err)
		}
	}

	return nil
}
