package domain

import (
	"context"
	"encoding/json"
	"errors"
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
	if event.EventType == "MATCH_ANSWER" {
		var payload EventPayload
		if err := json.Unmarshal([]byte(event.PayloadJSON), &payload); err != nil {
			return nil
		}
		if !payload.IsCorrect {
			return nil
		}

		// Paga uma vez por desafio. Rejogar um nó pagava de novo, e os portões da trilha
		// viravam moagem. Resposta sem `challenge_id` não tem como provar que é a
		// primeira, então não paga: é o evento de um app anterior a esta regra.
		if payload.ChallengeID == "" {
			return nil
		}

		// O nó e a trilha saem do banco, não do evento. Desafio que o banco não conhece
		// não paga: `user_paid_challenges` não tem chave estrangeira, e um id inventado
		// virava XP. Trilha paga fora da amostra só paga com direito ativo; o evento
		// segue na cadeia de qualquer jeito (spec, seção 8).
		var nodeID string
		var open, entitled bool
		err := tx.QueryRow(ctx, `
			SELECT n.id, `+openNode+`,
			       EXISTS (SELECT 1 FROM entitlements e
			               WHERE e.user_id = $2 AND e.track_id = n.track_id AND e.status = 'active')
			FROM challenges c
			JOIN skill_nodes n ON n.id = c.node_id
			JOIN tracks t ON t.id = n.track_id
			WHERE c.id = $1`, payload.ChallengeID, userID).Scan(&nodeID, &open, &entitled)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to read challenge track: %w", err)
		}
		if !open && !entitled {
			return nil
		}
		payload.NodeID = nodeID

		paid, err := tx.Exec(ctx, `
			INSERT INTO user_paid_challenges (user_id, challenge_id)
			VALUES ($1, $2)
			ON CONFLICT (user_id, challenge_id) DO NOTHING`,
			userID, payload.ChallengeID)
		if err != nil {
			return fmt.Errorf("failed to record paid challenge: %w", err)
		}
		if paid.RowsAffected() == 0 {
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
