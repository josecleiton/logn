package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
)

type EventPayload struct {
	IsCorrect   bool   `json:"is_correct"`
	ChallengeID string `json:"challenge_id"`
	NodeID      string `json:"node_id"`
}

func (r *Repository) ProcessEventXP(ctx context.Context, tx pgx.Tx, userID string, event GameEvent) error {
	if event.EventType != "challenge_answered" {
		return nil
	}

	var payload EventPayload
	if err := json.Unmarshal([]byte(event.PayloadJSON), &payload); err != nil {
		// Just ignore malformed payloads rather than failing the sync
		return nil
	}

	if !payload.IsCorrect || payload.NodeID == "" {
		return nil
	}

	// 1. Give XP (e.g., 50 XP per correct challenge)
	xpEarned := 50

	// 2. Upsert user progress
	upsertQuery := `
		INSERT INTO user_progress (user_id, node_id, current_xp, unlocked)
		VALUES ($1, $2, $3, false)
		ON CONFLICT (user_id, node_id) DO UPDATE 
		SET current_xp = user_progress.current_xp + EXCLUDED.current_xp`

	_, err := tx.Exec(ctx, upsertQuery, userID, payload.NodeID, xpEarned)
	if err != nil {
		return fmt.Errorf("failed to upsert progress: %w", err)
	}

	// 3. Check if unlocked
	checkQuery := `
		UPDATE user_progress up
		SET unlocked = true, completed_at = CURRENT_TIMESTAMP
		FROM skill_nodes sn
		WHERE up.node_id = sn.id 
		  AND up.user_id = $1 
		  AND up.node_id = $2 
		  AND up.current_xp >= sn.required_xp
		  AND up.unlocked = false`

	_, err = tx.Exec(ctx, checkQuery, userID, payload.NodeID)
	if err != nil {
		return fmt.Errorf("failed to unlock node: %w", err)
	}

	return nil
}
