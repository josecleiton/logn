package domain

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// O apelido do placar (docs/specs/logn_placar_spec.md, seção 5.3). É escolha única: a
// conta escolhe uma vez e não troca; só a moderação o tira, e aí a chance acaba.
var (
	ErrNicknameInvalid  = errors.New("nickname: formato inválido")
	ErrNicknameReserved = errors.New("nickname: reservado")
	ErrNicknameTaken    = errors.New("nickname: em uso")
	ErrNicknameLocked   = errors.New("nickname: a conta já escolheu ou perdeu a chance")
)

var nicknamePattern = regexp.MustCompile(`^[a-z0-9_]{3,20}$`)

// Nomes que ninguém escolhe, para ninguém se passar pelo app nem copiar o nome
// anônimo ("jogador_4821").
var reservedNicknames = []string{
	"logn", "admin", "administrador", "root", "gm", "mod", "moderador", "moderator",
	"staff", "equipe", "team", "suporte", "support", "oficial", "official",
	"sistema", "system", "jogador", "player", "jugador",
}

// NormalizeNickname tira os espaços das pontas, passa a minúsculas e confere o formato.
//
// Minúscula só de ASCII, byte a byte: strings.ToLower levaria o "K" do Kelvin (U+212A)
// a "k", e um apelido com letra de outro alfabeto passaria na regra que diz "letras sem
// acento".
func NormalizeNickname(raw string) (string, error) {
	s := []byte(strings.TrimSpace(raw))
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			s[i] = c + ('a' - 'A')
		}
	}
	n := string(s)
	if !nicknamePattern.MatchString(n) {
		return "", ErrNicknameInvalid
	}
	return n, nil
}

// reservedNickname diz se o apelido é um nome reservado, sozinho ou seguido só de `_` e
// dígitos (`gm`, `gm_1`, `logn2`). O nome no meio de outro passa (`modesto`).
func reservedNickname(n string) bool {
	for _, r := range reservedNicknames {
		if strings.HasPrefix(n, r) && strings.Trim(n[len(r):], "_0123456789") == "" {
			return true
		}
	}
	return false
}

// SetNickname grava o apelido da conta e devolve a forma normalizada.
//
// A conta que já escolheu, ou perdeu a chance, ouve "travada" antes de qualquer outra
// conferência: senão ela poderia sondar, sem fim, quais apelidos a moderação bloqueou.
// Depois vêm formato, reservado e bloqueado.
//
// O bloqueio entra no próprio UPDATE, e não numa leitura antes: um apelido moderado
// entre a leitura e a escrita escaparia. A unicidade fica com o índice: dois pedidos
// com o mesmo apelido ao mesmo tempo resolvem no banco, e um deles ouve "em uso".
func (r *Repository) SetNickname(ctx context.Context, userID, raw string) (string, error) {
	var locked bool
	if err := r.db.QueryRow(ctx, `
		SELECT nickname IS NOT NULL OR nickname_burned_at IS NOT NULL
		FROM users WHERE id = $1`, userID).Scan(&locked); err != nil {
		return "", err
	}
	if locked {
		return "", ErrNicknameLocked
	}
	n, err := NormalizeNickname(raw)
	if err != nil {
		return "", err
	}
	if reservedNickname(n) {
		return "", ErrNicknameReserved
	}

	tag, err := r.db.Exec(ctx, `
		UPDATE users SET nickname = $2
		WHERE id = $1 AND nickname IS NULL AND nickname_burned_at IS NULL
		  AND NOT EXISTS (SELECT 1 FROM blocked_nicknames WHERE nickname = $2)`, userID, n)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "users_nickname_key" {
		return "", ErrNicknameTaken
	}
	if err != nil {
		return "", err
	}
	if tag.RowsAffected() == 1 {
		return n, nil
	}
	// Nada gravou: ou o apelido está bloqueado, ou a conta travou entre a primeira
	// leitura e aqui.
	var blocked bool
	if err := r.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM blocked_nicknames WHERE nickname = $1)`, n).Scan(&blocked); err != nil {
		return "", err
	}
	if blocked {
		return "", ErrNicknameReserved
	}
	return "", ErrNicknameLocked
}
