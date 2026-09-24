package domain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository fala com o banco por um **pool**, não por uma conexão única.
//
// `*pgx.Conn` não é seguro para uso concorrente: dois handlers HTTP ao mesmo tempo
// derrubam um ao outro com `conn busy`. Como o servidor atende em paralelo por
// natureza, a conexão única fazia requisições legítimas falharem de forma
// intermitente — o verify de OTP recusava código válido.
type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Ping checks the database connectivity.
func (r *Repository) Ping(ctx context.Context) error {
	return r.db.Ping(ctx)
}

func (r *Repository) InsertChallenge(ctx context.Context, ch Challenge) error {
	// position_idx é NOT NULL desde que a ordem das letras virou dado em vez de efeito
	// do sort do id; origin é nula para o desafio que nasceu aqui.
	query := `INSERT INTO challenges (id, node_id, template_type, payload, position_idx, origin)
			  VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''))`
	_, err := r.db.Exec(ctx, query,
		ch.ID, ch.NodeID, ch.TemplateType, ch.Payload, ch.PositionIdx, ch.Origin)
	return err
}

// ErrStaleChain sinaliza que o topo da cadeia mudou entre a validação e a gravação:
// outro sync do mesmo usuário chegou antes. O cliente trata como rebase.
var ErrStaleChain = errors.New("sync chain moved concurrently")

// InsertSyncEvents grava os eventos e avança o topo da cadeia de expectedTop para
// newTopHash, desde que o topo ainda seja expectedTop.
//
// O upsert antigo sobrescrevia sem conferir. Dois syncs em paralelo validavam contra o
// mesmo topo, gravavam os dois, e o XP dos eventos entrava em dobro. Agora o segundo a
// chegar não acha o topo que validou e volta com ErrStaleChain.
func (r *Repository) InsertSyncEvents(ctx context.Context, payload SyncPayload, expectedTop, newTopHash string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// A linha não existir equivale a estar no gênesis. Se um sync concorrente a criou
	// primeiro, o `WHERE` do conflito não casa e nada é afetado.
	upsertUser := `INSERT INTO user_sync_state (user_id, last_hash) VALUES ($1, $2)
				   ON CONFLICT (user_id) DO UPDATE SET last_hash = EXCLUDED.last_hash
				   WHERE user_sync_state.last_hash = $3`
	tag, err := tx.Exec(ctx, upsertUser, payload.UserID, newTopHash, expectedTop)
	if err != nil {
		return fmt.Errorf("failed to upsert user_sync_state: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrStaleChain
	}

	for _, event := range payload.Events {
		query := `INSERT INTO game_events (id, user_id, event_type, payload_json, timestamp, previous_hash, current_hash)
				  VALUES ($1, $2, $3, $4, $5, $6, $7)`
		if err := r.ProcessEventXP(ctx, tx, payload.UserID, event); err != nil {
			return fmt.Errorf("failed to process XP for event %s: %w", event.ID, err)
		}

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
	// A coluna é node_id desde que a árvore virou DAG; `chapter` não existe e derrubava
	// a listagem inteira com um 500.
	//
	// A ordem define as letras A, B, C da partida: o core enumera esta lista já
	// ordenada. Ordenava por `id`, que é VARCHAR — ch_10 vinha antes de ch_2. Agora sai
	// de position_idx, que é dado explícito (ADR 0006).
	// position_idx vai junto: a coluna decide a ordem aqui, e o JSON a anunciava sem
	// nunca preenchê-la — a trilha empacotada saía com zero em todos os desafios.
	query := `SELECT id, node_id, template_type, payload, position_idx, COALESCE(origin, '')
			  FROM challenges ORDER BY node_id, position_idx`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var challenges []Challenge
	for rows.Next() {
		var ch Challenge
		if err := rows.Scan(&ch.ID, &ch.NodeID, &ch.TemplateType, &ch.Payload, &ch.PositionIdx, &ch.Origin); err != nil {
			return nil, err
		}
		challenges = append(challenges, ch)
	}
	return challenges, nil
}

type User struct {
	ID           string
	Email        string
	PasswordHash string
}

type RefreshToken struct {
	ID        string
	UserID    string
	TokenHash string
	Revoked   bool
	ExpiresAt time.Time
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	query := `SELECT id, email, password_hash FROM users WHERE email = $1`
	var u User
	var pwHash *string
	err := r.db.QueryRow(ctx, query, email).Scan(&u.ID, &u.Email, &pwHash)
	if err != nil {
		return nil, err
	}
	if pwHash != nil {
		u.PasswordHash = *pwHash
	}
	return &u, nil
}

func (r *Repository) CreateRefreshToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	query := `INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`
	_, err := r.db.Exec(ctx, query, userID, tokenHash, expiresAt)
	return err
}

func (r *Repository) GetRefreshToken(ctx context.Context, tokenHash string) (*RefreshToken, error) {
	query := `SELECT id, user_id, token_hash, revoked, expires_at FROM refresh_tokens WHERE token_hash = $1`
	var t RefreshToken
	err := r.db.QueryRow(ctx, query, tokenHash).Scan(&t.ID, &t.UserID, &t.TokenHash, &t.Revoked, &t.ExpiresAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// RotateRefreshToken revoga o token apresentado e emite outro, na mesma transação.
//
// Sem rotação o refresh só renovava o access token, e o prazo do refresh seguia
// correndo desde o login: quem usava o app todo dia era deslogado no trigésimo
// primeiro. Rotacionar também dá detecção de reuso de graça — um token revogado
// apresentado de novo é sinal de cópia, e o `Revoked` já barra.
func (r *Repository) RotateRefreshToken(
	ctx context.Context, oldHash, userID, newHash string, expiresAt time.Time,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Só revoga o que ainda valia: duas chamadas com o mesmo token não podem as duas
	// emitir um token novo.
	tag, err := tx.Exec(ctx,
		`UPDATE refresh_tokens SET revoked = TRUE WHERE token_hash = $1 AND revoked = FALSE`,
		oldHash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		// Token já trocado apareceu de novo: um dos dois portadores é cópia, e daqui
		// não dá para saber qual. Derruba todas as sessões do usuário, como pede o
		// OAuth BCP; o dono de verdade faz login outra vez, e quem copiou perde o acesso.
		if _, err := r.db.Exec(ctx,
			`UPDATE refresh_tokens SET revoked = TRUE WHERE user_id = $1 AND revoked = FALSE`,
			userID); err != nil {
			return err
		}
		return ErrRefreshTokenAlreadyUsed
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, newHash, expiresAt); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

type SkillNode struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Row           int      `json:"row"`
	Column        int      `json:"column"`
	RequiredXP    int      `json:"required_xp"`
	Prerequisites []string `json:"prerequisites"`
}

type UserProgress struct {
	NodeID      string     `json:"node_id"`
	CurrentXP   int        `json:"current_xp"`
	Unlocked    bool       `json:"unlocked"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// UserStats é o que o cliente precisa para reconstruir a barra de nível ao abrir.
//
// Sem isto o XP vivia só na memória do app: sincronizava tudo certinho e, no login
// seguinte, o jogador voltava para zero com os eventos parados no servidor.
type UserStats struct {
	GlobalXP         int            `json:"global_xp"`
	BugsFound        int            `json:"bugs_found"`
	DryRunsCompleted int            `json:"dry_runs_completed"`
	Nodes            []UserProgress `json:"nodes"`
	// Desafios que já renderam XP. O cliente precisa deles para não prometer XP de novo
	// por um desafio que o servidor não vai pagar, inclusive depois de trocar de aparelho.
	PaidChallengeIDs []string `json:"paid_challenge_ids"`
}

func (r *Repository) GetUserStats(ctx context.Context, userID string) (UserStats, error) {
	var stats UserStats

	query := `SELECT global_xp, bugs_found, dry_runs_completed FROM users WHERE id = $1`
	err := r.db.QueryRow(ctx, query, userID).
		Scan(&stats.GlobalXP, &stats.BugsFound, &stats.DryRunsCompleted)
	if err != nil {
		return stats, err
	}

	nodes, err := r.GetUserProgress(ctx, userID)
	if err != nil {
		return stats, err
	}
	// Nunca `null` no JSON: o cliente desserializa em lista.
	stats.Nodes = nodes
	if stats.Nodes == nil {
		stats.Nodes = []UserProgress{}
	}

	rows, err := r.db.Query(ctx,
		`SELECT challenge_id FROM user_paid_challenges WHERE user_id = $1 ORDER BY challenge_id`, userID)
	if err != nil {
		return stats, err
	}
	defer rows.Close()
	stats.PaidChallengeIDs = []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return stats, err
		}
		stats.PaidChallengeIDs = append(stats.PaidChallengeIDs, id)
	}
	return stats, rows.Err()
}

func (r *Repository) GetSkillNodes(ctx context.Context) ([]SkillNode, error) {
	query := `SELECT id, name, description, row_idx, col_idx, required_xp, prerequisites FROM skill_nodes ORDER BY row_idx ASC`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []SkillNode
	for rows.Next() {
		var n SkillNode
		var prereqsJSON []byte
		if err := rows.Scan(&n.ID, &n.Name, &n.Description, &n.Row, &n.Column, &n.RequiredXP, &prereqsJSON); err != nil {
			return nil, err
		}
		json.Unmarshal(prereqsJSON, &n.Prerequisites)
		if n.Prerequisites == nil {
			n.Prerequisites = []string{}
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}

func (r *Repository) GetUserProgress(ctx context.Context, userID string) ([]UserProgress, error) {
	query := `SELECT node_id, current_xp, unlocked, completed_at FROM user_progress WHERE user_id = $1`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var progress []UserProgress
	for rows.Next() {
		var p UserProgress
		if err := rows.Scan(&p.NodeID, &p.CurrentXP, &p.Unlocked, &p.CompletedAt); err != nil {
			return nil, err
		}
		progress = append(progress, p)
	}
	return progress, nil
}

func (r *Repository) CreateUser(ctx context.Context, email, passwordHash string, ageConfirmed bool, legalAcceptances []string) (string, error) {
	var id string
	
	legalJson, _ := json.Marshal(legalAcceptances)
	if legalJson == nil {
		legalJson = []byte("[]")
	}

	query := `
		INSERT INTO users (email, password_hash, age_confirmed_at, legal_acceptances)
		VALUES ($1, $2, CASE WHEN $3::boolean THEN CURRENT_TIMESTAMP ELSE NULL END, $4)
		RETURNING id
	`
	err := r.db.QueryRow(ctx, query, email, passwordHash, ageConfirmed, legalJson).Scan(&id)
	return id, err
}

// ErrUserNotFound sinaliza e-mail sem conta.
var ErrUserNotFound = errors.New("user not found")

// ResetUserPassword troca a senha e derruba todas as sessões abertas, na mesma
// transação, e devolve o id do dono.
//
// Antes só trocava o hash. Quem tinha roubado a conta seguia com o refresh token, e a
// rotação o mantinha vivo para sempre: a troca de senha não expulsava ninguém.
func (r *Repository) ResetUserPassword(ctx context.Context, email, passwordHash string) (string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	var userID string
	err = tx.QueryRow(ctx,
		`UPDATE users SET password_hash = $1 WHERE email = $2 RETURNING id`,
		passwordHash, email).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrUserNotFound
	}
	if err != nil {
		return "", err
	}

	if _, err := tx.Exec(ctx,
		`UPDATE refresh_tokens SET revoked = TRUE WHERE user_id = $1 AND revoked = FALSE`,
		userID); err != nil {
		return "", err
	}

	return userID, tx.Commit(ctx)
}

// UpdatePasswordHash regrava o hash de quem acabou de acertar a senha. É o que leva
// hashes antigos para os parâmetros atuais do Argon2 sem pedir nada ao usuário.
func (r *Repository) UpdatePasswordHash(ctx context.Context, userID, passwordHash string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET password_hash = $1 WHERE id = $2`, passwordHash, userID)
	return err
}

func (r *Repository) GetUserByID(ctx context.Context, userID string) (*User, error) {
	query := `
		SELECT id, email, password_hash
		FROM users
		WHERE id = $1
	`
	row := r.db.QueryRow(ctx, query, userID)
	var user User
	err := row.Scan(&user.ID, &user.Email, &user.PasswordHash)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) MarkAccountForDeletion(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET deletion_requested_at = CURRENT_TIMESTAMP WHERE id = $1`, userID)
	return err
}

func (r *Repository) IsUserActive(ctx context.Context, userID string) bool {
	var deletionRequestedAt *time.Time
	err := r.db.QueryRow(ctx, "SELECT deletion_requested_at FROM users WHERE id = $1", userID).Scan(&deletionRequestedAt)
	if err != nil {
		return false
	}
	return deletionRequestedAt == nil
}

// PurgeDeletedAccounts apaga contas que pediram exclusão há mais de 30 dias.
// Apaga os dados em cascata manualmente para game_events e user_sync_state.
func (r *Repository) PurgeDeletedAccounts(ctx context.Context) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Seleciona usuários a apagar
	query := `
		SELECT id FROM users
		WHERE deletion_requested_at IS NOT NULL
		  AND deletion_requested_at < CURRENT_TIMESTAMP - INTERVAL '30 days'
	`
	rows, err := tx.Query(ctx, query)
	if err != nil {
		return err
	}

	var userIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		userIDs = append(userIDs, id)
	}
	rows.Close()

	if len(userIDs) == 0 {
		return nil
	}

	for _, uid := range userIDs {
		// game_events depende de user_sync_state, que logicamente depende de users
		_, err = tx.Exec(ctx, "DELETE FROM game_events WHERE user_id = $1", uid)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "DELETE FROM user_sync_state WHERE user_id = $1", uid)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "DELETE FROM user_progress WHERE user_id = $1", uid)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "DELETE FROM refresh_tokens WHERE user_id = $1", uid)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "DELETE FROM users WHERE id = $1", uid)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}
