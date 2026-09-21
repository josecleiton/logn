package domain

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type Repository struct {
	db *pgx.Conn
}

func NewRepository(db *pgx.Conn) *Repository {
	return &Repository{db: db}
}

func (r *Repository) InsertChallenge(ctx context.Context, ch Challenge) error {
	query := `INSERT INTO challenges (id, chapter, template_type, version, payload)
			  VALUES ($1, $2, $3, $4, $5)`
	_, err := r.db.Exec(ctx, query, ch.ID, ch.Chapter, ch.TemplateType, ch.Version, ch.Payload)
	return err
}

func (r *Repository) InsertSyncEvents(ctx context.Context, payload SyncPayload, newTopHash string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Update user top hash (upsert)
	upsertUser := `INSERT INTO user_sync_state (user_id, last_hash) VALUES ($1, $2)
				   ON CONFLICT (user_id) DO UPDATE SET last_hash = EXCLUDED.last_hash`
	if _, err := tx.Exec(ctx, upsertUser, payload.UserID, newTopHash); err != nil {
		return fmt.Errorf("failed to upsert user_sync_state: %w", err)
	}

	for _, event := range payload.Events {
		query := `INSERT INTO game_events (id, user_id, event_type, payload_json, timestamp, previous_hash, current_hash)
				  VALUES ($1, $2, $3, $4, $5, $6, $7)`
		if _, err := tx.Exec(ctx, query, event.ID, payload.UserID, event.EventType, event.PayloadJSON, event.Timestamp, event.PreviousHash, event.CurrentHash); err != nil {
			return fmt.Errorf("failed to insert event %s: %w", event.ID, err)
		}
	}

	return tx.Commit(ctx)
}

func (r *Repository) GetUserLastHash(ctx context.Context, userID string) (string, error) {
	var hash string
	err := r.db.QueryRow(ctx, "SELECT last_hash FROM user_sync_state WHERE user_id = $1", userID).Scan(&hash)
	if err != nil {
		if err == pgx.ErrNoRows {
			// Se o usuário não existir, retornamos a hash vazia (gênesis)
			return "0000000000000000000000000000000000000000000000000000000000000000", nil
		}
		return "", err
	}
	return hash, nil
}

func (r *Repository) GetChallenges(ctx context.Context) ([]Challenge, error) {
	query := `SELECT id, chapter, template_type, version, payload FROM challenges ORDER BY id ASC`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var challenges []Challenge
	for rows.Next() {
		var ch Challenge
		if err := rows.Scan(&ch.ID, &ch.Chapter, &ch.TemplateType, &ch.Version, &ch.Payload); err != nil {
			return nil, err
		}
		challenges = append(challenges, ch)
	}
	return challenges, nil
}
