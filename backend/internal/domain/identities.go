package domain

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// GetUserIDByIdentity devolve a conta ligada à identidade externa, ou ErrUserNotFound.
func (r *Repository) GetUserIDByIdentity(ctx context.Context, provider, subject string) (string, error) {
	var userID string
	err := r.db.QueryRow(ctx,
		`SELECT user_id FROM user_identities WHERE provider = $1 AND subject = $2`,
		provider, subject).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrUserNotFound
	}
	return userID, err
}

// LinkIdentity liga a identidade à conta e devolve o dono dela depois disso.
//
// Dois logins simultâneos do mesmo `sub` correm para o mesmo INSERT. O que perde não
// falha: lê o dono que o outro gravou, e quem chama entra na conta dele, não na que
// tinha escolhido.
func (r *Repository) LinkIdentity(ctx context.Context, provider, subject, userID string) (string, error) {
	var owner string
	err := r.db.QueryRow(ctx, `
		WITH ins AS (
			INSERT INTO user_identities (provider, subject, user_id)
			VALUES ($1, $2, $3)
			ON CONFLICT (provider, subject) DO NOTHING
			RETURNING user_id
		)
		SELECT user_id FROM ins
		UNION ALL
		SELECT user_id FROM user_identities WHERE provider = $1 AND subject = $2
		LIMIT 1`,
		provider, subject, userID).Scan(&owner)
	// O INSERT esperou o outro login gravar e desistiu, mas o SELECT ainda lê a foto de
	// antes, sem a linha dele. Uma leitura nova já a vê.
	if errors.Is(err, pgx.ErrNoRows) {
		return r.GetUserIDByIdentity(ctx, provider, subject)
	}
	return owner, err
}

// CreateSocialUser cria a conta sem senha, com idade, país e aceites, e a liga à
// identidade externa, numa transação só. Conta sem identidade seria uma conta em que
// ninguém consegue entrar.
func (r *Repository) CreateSocialUser(ctx context.Context, email, provider, subject string, ageConfirmed bool, country string, acceptances []LegalAcceptance) (string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	id, err := createUserTx(ctx, tx, email, "", ageConfirmed, country, acceptances)
	if err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO user_identities (provider, subject, user_id) VALUES ($1, $2, $3)`,
		provider, subject, id); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return id, nil
}

// HasIdentity diz se a identidade externa é desta conta. É a prova de dono que a
// exclusão pede a quem não tem senha.
func (r *Repository) HasIdentity(ctx context.Context, userID, provider, subject string) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM user_identities
			WHERE user_id = $1 AND provider = $2 AND subject = $3
		)`, userID, provider, subject).Scan(&ok)
	return ok, err
}
