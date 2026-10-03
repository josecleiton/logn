package domain

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// O placar geral de XP (docs/specs/logn_placar_spec.md).
const (
	// Quantas contas visíveis abrem o placar. Abaixo disso a lista não sai, e ninguém
	// recebe posição.
	LeaderboardOpenAt = 10
	// Quantas linhas a lista traz.
	LeaderboardTop = 100
)

// LeaderboardRow é uma linha da lista. O nome sai como o número ou o apelido; quem
// compõe "jogador #N" é o app, com o catálogo.
type LeaderboardRow struct {
	Rank       int     `json:"rank"`
	AnonNumber int     `json:"anon_number"`
	Nickname   *string `json:"nickname"`
	XP         int     `json:"xp"`
	IsMe       bool    `json:"is_me"`
}

// LeaderboardMe é a linha de quem pediu. `Rank` é nulo com o placar fechado, com XP
// zero ou com a conta oculta.
type LeaderboardMe struct {
	Rank       *int    `json:"rank"`
	AnonNumber int     `json:"anon_number"`
	Nickname   *string `json:"nickname"`
	XP         int     `json:"xp"`
	Hidden     bool    `json:"hidden"`
}

// Leaderboard é o corpo de GET /api/v1/leaderboard. Não leva id nem e-mail de ninguém.
type Leaderboard struct {
	Open      bool             `json:"open"`
	Missing   int              `json:"missing"`
	Threshold int              `json:"threshold"`
	Rows      []LeaderboardRow `json:"rows"`
	Me        LeaderboardMe    `json:"me"`
	// Segundos desde a época, a hora do servidor que vira o "atualizado há X".
	GeneratedAt int64 `json:"generated_at"`
}

// GetLeaderboard monta o placar para a conta `userID`.
//
// Visível é quem tem XP da trilha gratuita, não pediu exclusão e não foi ocultado. A
// posição é row_number() na ordem do placar (XP, quem chegou primeiro, id): única, e
// por definição "visíveis à frente, mais um". As duas leituras rodam na mesma foto do
// banco, para a linha do jogador não discordar da lista.
func (r *Repository) GetLeaderboard(ctx context.Context, userID string) (Leaderboard, error) {
	return r.leaderboardOpeningAt(ctx, userID, LeaderboardOpenAt)
}

// leaderboardOpeningAt é o placar com o limiar de abertura dado. Só os testes passam
// outro valor que não LeaderboardOpenAt, para provar contra o banco o placar fechado,
// que o banco local, com mais de dez contas, não deixaria ver.
func (r *Repository) leaderboardOpeningAt(ctx context.Context, userID string, openAt int) (Leaderboard, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Leaderboard{}, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		WITH ranked AS (
			SELECT id, anon_number, nickname, free_xp,
			       row_number() OVER (ORDER BY free_xp DESC, free_xp_reached_at, id) AS rank,
			       count(*) OVER () AS total
			FROM users
			WHERE free_xp > 0 AND deletion_requested_at IS NULL AND NOT leaderboard_hidden
		)
		SELECT rank, anon_number, nickname, free_xp, id = $1::uuid, total
		FROM ranked
		WHERE rank <= $2 OR id = $1::uuid
		ORDER BY rank`, userID, LeaderboardTop)
	if err != nil {
		return Leaderboard{}, err
	}
	board := Leaderboard{Threshold: openAt, Rows: []LeaderboardRow{}}
	var total int
	var myRank *int
	for rows.Next() {
		var row LeaderboardRow
		if err := rows.Scan(&row.Rank, &row.AnonNumber, &row.Nickname, &row.XP, &row.IsMe, &total); err != nil {
			rows.Close()
			return Leaderboard{}, err
		}
		if row.IsMe {
			rank := row.Rank
			myRank = &rank
		}
		// A linha do jogador além do top só volta para dar a posição dele.
		if row.Rank <= LeaderboardTop {
			board.Rows = append(board.Rows, row)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Leaderboard{}, err
	}

	err = tx.QueryRow(ctx, `
		SELECT anon_number, nickname, free_xp, leaderboard_hidden
		FROM users WHERE id = $1`, userID).Scan(
		&board.Me.AnonNumber, &board.Me.Nickname, &board.Me.XP, &board.Me.Hidden)
	// A conta sumiu entre a autenticação e esta leitura: é sessão que não vale mais.
	if errors.Is(err, pgx.ErrNoRows) {
		return Leaderboard{}, ErrUserNotFound
	}
	if err != nil {
		return Leaderboard{}, err
	}

	settleLeaderboard(&board, total, myRank, openAt)
	board.GeneratedAt = time.Now().Unix()
	return board, nil
}

// settleLeaderboard decide se o placar abre. Fechado, nenhuma posição sai: nem a lista,
// nem a do jogador, só quantos faltam.
func settleLeaderboard(board *Leaderboard, total int, myRank *int, openAt int) {
	board.Open = total >= openAt
	if board.Open {
		board.Missing = 0
		board.Me.Rank = myRank
		return
	}
	board.Missing = openAt - total
	board.Rows = []LeaderboardRow{}
	board.Me.Rank = nil
}
