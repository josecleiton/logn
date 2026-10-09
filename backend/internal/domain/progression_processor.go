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

// acceptedAnswer é a primeira resposta certa de um desafio no lote. Só ela pode pagar.
type acceptedAnswer struct {
	challengeID string
	template    string
}

// challengeTrack é o que o banco diz do desafio: de que nó é e se a conta recebe XP por
// ele.
type challengeTrack struct {
	nodeID string
	pays   bool
	isFree bool
}

// ProcessEventsXP paga o XP das respostas certas do lote, dentro da transação do sync.
//
// Era um evento por vez, com até seis comandos cada: um lote de 200 eventos fazia mais de
// mil idas ao banco segurando a conexão e a linha de `users`. Agora são cinco, qualquer
// que seja o tamanho do lote, e o resultado é o mesmo do laço: cada desafio paga uma vez,
// pela primeira resposta certa, e o `CURRENT_TIMESTAMP` é o da transação nos dois casos.
func (r *Repository) ProcessEventsXP(ctx context.Context, tx pgx.Tx, userID string, events []GameEvent) error {
	answers := acceptedAnswers(events)
	if len(answers) == 0 {
		return nil
	}

	tracks, err := challengeTracks(ctx, tx, userID, answers)
	if err != nil {
		return err
	}

	var payable []string
	for _, a := range answers {
		if tracks[a.challengeID].pays {
			payable = append(payable, a.challengeID)
		}
	}
	if len(payable) == 0 {
		return nil
	}

	// O que já estava em `user_paid_challenges` não volta no RETURNING: replay e
	// rejogo de nó não pagam de novo.
	rows, err := tx.Query(ctx, `
		INSERT INTO user_paid_challenges (user_id, challenge_id)
		SELECT $1::uuid, unnest($2::text[])
		ON CONFLICT (user_id, challenge_id) DO NOTHING
		RETURNING challenge_id`, userID, payable)
	if err != nil {
		return fmt.Errorf("failed to record paid challenges: %w", err)
	}
	paid, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return fmt.Errorf("failed to record paid challenges: %w", err)
	}
	if len(paid) == 0 {
		return nil
	}
	paidNow := make(map[string]bool, len(paid))
	for _, id := range paid {
		paidNow[id] = true
	}

	var xp, bugs, dryRuns, freeXP int
	var nodes []string
	nodeXP := map[string]int{}
	for _, a := range answers {
		if !paidNow[a.challengeID] {
			continue
		}
		track := tracks[a.challengeID]
		xp += XPPerAcceptedAnswer
		if a.template == "SPOT_THE_BUG" {
			bugs++
		}
		// Dry run é o traçado de código aceito, não o fim da partida. Estava somando
		// um a cada MATCH_END: quem jogasse sem tocar num DRY_RUN ganhava o número.
		if a.template == "DRY_RUN" {
			dryRuns++
		}
		// O placar conta só a trilha gratuita (docs/specs/logn_placar_spec.md).
		if track.isFree {
			freeXP += XPPerAcceptedAnswer
		}
		if _, ok := nodeXP[track.nodeID]; !ok {
			nodes = append(nodes, track.nodeID)
		}
		nodeXP[track.nodeID] += XPPerAcceptedAnswer
	}
	xps := make([]int, len(nodes))
	for i, n := range nodes {
		xps[i] = nodeXP[n]
	}

	// Os três seguem numa ida só. A hora em que o XP gratuito chegou ao valor é a do
	// servidor, não a do evento: a do evento vem do cliente e daria para forjar o
	// desempate. É o mesmo CURRENT_TIMESTAMP das linhas de `user_paid_challenges`, e a
	// migração 0071 conta com essa igualdade.
	batch := &pgx.Batch{}
	batch.Queue(`
		UPDATE users
		SET global_xp = global_xp + $1,
		    bugs_found = bugs_found + $2,
		    dry_runs_completed = dry_runs_completed + $3,
		    free_xp = free_xp + $4,
		    free_xp_reached_at = CASE WHEN $4::int > 0 THEN CURRENT_TIMESTAMP ELSE free_xp_reached_at END
		WHERE id = $5`, xp, bugs, dryRuns, freeXP, userID)
	// Agregado por nó antes: o ON CONFLICT não deixa o mesmo comando tocar a linha duas
	// vezes.
	batch.Queue(`
		INSERT INTO user_progress (user_id, node_id, current_xp, unlocked)
		SELECT $1::uuid, p.node_id, p.xp, false
		FROM unnest($2::uuid[], $3::int[]) AS p(node_id, xp)
		ON CONFLICT (user_id, node_id) DO UPDATE
		SET current_xp = user_progress.current_xp + EXCLUDED.current_xp`, userID, nodes, xps)
	batch.Queue(`
		UPDATE user_progress up
		SET unlocked = true, completed_at = CURRENT_TIMESTAMP
		FROM skill_nodes sn
		WHERE up.node_id = sn.id
		  AND up.user_id = $1::uuid
		  AND up.node_id = ANY($2::uuid[])
		  AND up.current_xp >= sn.required_xp
		  AND up.unlocked = false`, userID, nodes)
	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		return fmt.Errorf("failed to update stats and progress: %w", err)
	}
	return nil
}

// acceptedAnswers tira do lote as respostas que podem pagar, na ordem, uma por desafio.
//
// Resposta sem `challenge_id` não tem como provar que é a primeira, então não paga: é o
// evento de um app anterior a esta regra. Payload que não decodifica também não.
func acceptedAnswers(events []GameEvent) []acceptedAnswer {
	var answers []acceptedAnswer
	seen := map[string]bool{}
	for _, event := range events {
		if event.EventType != "MATCH_ANSWER" {
			continue
		}
		var payload EventPayload
		if err := json.Unmarshal([]byte(event.PayloadJSON), &payload); err != nil {
			continue
		}
		if !payload.IsCorrect || payload.ChallengeID == "" || seen[payload.ChallengeID] {
			continue
		}
		seen[payload.ChallengeID] = true
		answers = append(answers, acceptedAnswer{challengeID: payload.ChallengeID, template: payload.TemplateType})
	}
	return answers
}

// challengeTracks lê do banco, numa consulta, o nó e a trilha de cada desafio do lote.
//
// O nó sai daqui, não do evento. Desafio que o banco não conhece fica fora do mapa e
// não paga: `user_paid_challenges` não tem chave estrangeira, e um id inventado virava
// XP. Trilha paga fora da amostra só paga com direito ativo, e a amostra de trilha
// indisponível só a quem a vê (ADR 0014); o evento segue na cadeia de qualquer jeito
// (spec, seção 8).
func challengeTracks(ctx context.Context, tx pgx.Tx, userID string, answers []acceptedAnswer) (map[string]challengeTrack, error) {
	ids := make([]string, len(answers))
	for i, a := range answers {
		ids[i] = a.challengeID
	}
	rows, err := tx.Query(ctx, `
		SELECT c.id, n.id, `+openNode+` AND `+visibleTrack("$2")+`,
		       EXISTS (SELECT 1 FROM entitlements e
		               WHERE e.user_id = $2::uuid AND e.track_id = n.track_id AND e.status = 'active'),
		       t.kind = 'free'
		FROM challenges c
		JOIN skill_nodes n ON n.id = c.node_id
		JOIN tracks t ON t.id = n.track_id
		WHERE c.id = ANY($1::text[])`, ids, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to read challenge tracks: %w", err)
	}
	defer rows.Close()

	tracks := make(map[string]challengeTrack, len(ids))
	for rows.Next() {
		var id string
		var track challengeTrack
		var open, entitled bool
		if err := rows.Scan(&id, &track.nodeID, &open, &entitled, &track.isFree); err != nil {
			return nil, fmt.Errorf("failed to read challenge tracks: %w", err)
		}
		track.pays = open || entitled
		tracks[id] = track
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read challenge tracks: %w", err)
	}
	return tracks, nil
}
