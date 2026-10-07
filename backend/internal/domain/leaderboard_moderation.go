package domain

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// Moderação do placar e objeção ao placar (docs/specs/logn_placar_spec.md, seção 8),
// pela rota interna, como a revogação manual (ADR 0021).
const (
	// Volta o apelido ao número, queima a chance e bloqueia o apelido para todo mundo.
	LeaderboardAnonymize = "anonymize"
	// Tira a conta do placar; é a objeção que chega por e-mail.
	LeaderboardHide = "hide"
	// Desfaz o hide.
	LeaderboardUnhide = "unhide"
)

var (
	ErrInvalidLeaderboardAction = errors.New("invalid leaderboard action")
	ErrLeaderboardNoChange      = errors.New("the action changes nothing on this account")
)

// LeaderboardTarget é como quem opera acha a conta: exatamente um dos campos. A objeção
// chega por e-mail, e o que se vê no placar é o apelido ou o número.
type LeaderboardTarget struct {
	UserID     string
	Nickname   string
	AnonNumber int
	Email      string
}

// LeaderboardAction é um pedido da rota interna. `Actor` é a conta de serviço do token.
type LeaderboardAction struct {
	Target LeaderboardTarget
	Action string
	Reason string
	Actor  string
}

// leaderboardTargetQuery escolhe a coluna da busca, de uma lista fechada. O valor vai
// sempre como parâmetro.
func leaderboardTargetQuery(t LeaderboardTarget) (string, any, error) {
	set := 0
	var column string
	var value any
	if t.UserID != "" {
		set++
		column, value = "id = $1::uuid", t.UserID
	}
	if t.Nickname != "" {
		set++
		n, err := NormalizeNickname(t.Nickname)
		if err != nil {
			return "", nil, ErrInvalidLeaderboardAction
		}
		column, value = "nickname = $1", n
	}
	if t.AnonNumber != 0 {
		set++
		column, value = "anon_number = $1", t.AnonNumber
	}
	if t.Email != "" {
		set++
		column, value = "email = $1", strings.ToLower(strings.TrimSpace(t.Email))
	}
	if set != 1 {
		return "", nil, ErrInvalidLeaderboardAction
	}
	return column, value, nil
}

// ApplyLeaderboardAction aplica a ação e grava o histórico na mesma transação. Devolve
// o id da conta, para o log.
func (r *Repository) ApplyLeaderboardAction(ctx context.Context, a LeaderboardAction) (string, error) {
	switch a.Action {
	case LeaderboardAnonymize, LeaderboardHide, LeaderboardUnhide:
	default:
		return "", ErrInvalidLeaderboardAction
	}
	reason := strings.TrimSpace(a.Reason)
	// O Postgres recusa NUL em texto: sem esta conferência, o motivo com NUL virava 500.
	if reason == "" || len([]rune(reason)) > 2000 || a.Actor == "" ||
		!utf8.ValidString(reason) || strings.ContainsRune(reason, 0) {
		return "", ErrInvalidLeaderboardAction
	}
	where, value, err := leaderboardTargetQuery(a.Target)
	if err != nil {
		return "", err
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	var userID string
	var nickname *string
	var burned, hidden bool
	err = tx.QueryRow(ctx, `
		SELECT id, nickname, nickname_burned_at IS NOT NULL, leaderboard_hidden
		FROM users WHERE `+where+` FOR UPDATE`, value).Scan(&userID, &nickname, &burned, &hidden)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrUserNotFound
	}
	if err != nil {
		return "", err
	}

	switch a.Action {
	case LeaderboardAnonymize:
		if nickname == nil && burned {
			return "", ErrLeaderboardNoChange
		}
		if _, err := tx.Exec(ctx, `
			UPDATE users SET nickname = NULL, nickname_burned_at = COALESCE(nickname_burned_at, CURRENT_TIMESTAMP)
			WHERE id = $1`, userID); err != nil {
			return "", err
		}
		if nickname != nil {
			if _, err := tx.Exec(ctx,
				`INSERT INTO blocked_nicknames (nickname) VALUES ($1) ON CONFLICT DO NOTHING`, *nickname); err != nil {
				return "", err
			}
		}
	case LeaderboardHide, LeaderboardUnhide:
		hide := a.Action == LeaderboardHide
		if hidden == hide {
			return "", ErrLeaderboardNoChange
		}
		if _, err := tx.Exec(ctx, `UPDATE users SET leaderboard_hidden = $2 WHERE id = $1`, userID, hide); err != nil {
			return "", err
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO leaderboard_actions (user_id, action, nickname, reason, actor)
		VALUES ($1, $2, $3, $4, $5)`, userID, a.Action, nickname, reason, a.Actor); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return userID, nil
}
