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

type LegalDocument struct {
	ID          string
	Kind        string
	Locale      string
	Version     int
	EffectiveAt time.Time
	Material    bool
	BodyHTML    string
	CreatedAt   time.Time
}

type LegalAcceptance struct {
	Kind    string `json:"kind"`
	Version int    `json:"version"`
	Locale  string `json:"locale"`
}

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

// publishedNodes é a regra de publicação por língua, como CTE: um nó aparece na língua
// $1 quando tem o próprio nome nela e nenhum desafio dele está sem tradução nela.
//
// Não há coluna de "publicado". A tradução que falta esconde o nó inteiro, e não um
// desafio solto: a partida é por nó, e um nó pela metade mudaria as letras A, B, C.
const publishedNodes = `
	WITH publicados AS (
		SELECT n.id FROM skill_nodes n
		JOIN skill_node_translations nt ON nt.node_id = n.id AND nt.locale = $1
		WHERE NOT EXISTS (
			SELECT 1 FROM challenges c
			WHERE c.node_id = n.id AND NOT EXISTS (
				SELECT 1 FROM challenge_translations t
				WHERE t.challenge_id = c.id AND t.locale = $1)))`

// GetChallenges devolve os desafios dos nós publicados na língua, com o texto dela.
func (r *Repository) GetChallenges(ctx context.Context, locale string) ([]Challenge, error) {
	// A ordem define as letras A, B, C da partida: o core enumera esta lista já
	// ordenada. Ordenava por `id`, que é VARCHAR — ch_10 vinha antes de ch_2. Agora sai
	// de position_idx, que é dado explícito (ADR 0006).
	query := publishedNodes + `
		SELECT c.id, c.node_id, c.template_type, c.payload, c.position_idx, COALESCE(c.origin, ''),
		       t.title, t.description, t.explanation, COALESCE(t.watch_note, ''), t.option_labels
		FROM challenges c
		JOIN publicados p ON p.id = c.node_id
		JOIN challenge_translations t ON t.challenge_id = c.id AND t.locale = $1
		ORDER BY c.node_id, c.position_idx`
	rows, err := r.db.Query(ctx, query, locale)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Nunca `null` no JSON: o Core desserializa em lista e recusaria a resposta inteira.
	challenges := []Challenge{}
	for rows.Next() {
		var ch Challenge
		var text ChallengeText
		var labels []byte
		if err := rows.Scan(&ch.ID, &ch.NodeID, &ch.TemplateType, &ch.Payload, &ch.PositionIdx, &ch.Origin,
			&text.Title, &text.Description, &text.Explanation, &text.WatchNote, &labels); err != nil {
			return nil, err
		}
		if len(labels) > 0 {
			if err := json.Unmarshal(labels, &text.OptionLabels); err != nil {
				return nil, fmt.Errorf("rótulos de %s/%s: %w", ch.ID, locale, err)
			}
		}
		payload, err := AssemblePayload(ch.Payload, text)
		if err != nil {
			return nil, fmt.Errorf("desafio %s: %w", ch.ID, err)
		}
		ch.Payload = payload
		challenges = append(challenges, ch)
	}
	return challenges, rows.Err()
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

// OriginCard é o texto do selo de origem, na língua pedida: de onde um desafio veio,
// quando não foi escrito para o LogN. O cliente troca a string `origin` do desafio pelo
// cartão com este id.
type OriginCard struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
	Body string `json:"body"`
}

// GetOriginCards devolve os cartões de origem na língua pedida. Uma origem sem
// tradução nela fica de fora, como um nó sem tradução fica de fora de GetSkillNodes; o
// cliente que não achar o cartão pelo id abre um com só o id como nome, mas isso é
// decisão do Core, não do servidor.
func (r *Repository) GetOriginCards(ctx context.Context, locale string) ([]OriginCard, error) {
	query := `
		SELECT o.id, t.name, t.role, t.body
		FROM challenge_origins o
		JOIN challenge_origin_translations t ON t.origin_id = o.id AND t.locale = $1
		ORDER BY o.id`
	rows, err := r.db.Query(ctx, query, locale)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cards := []OriginCard{}
	for rows.Next() {
		var c OriginCard
		if err := rows.Scan(&c.ID, &c.Name, &c.Role, &c.Body); err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	return cards, rows.Err()
}

type SkillNode struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Row           int      `json:"row"`
	Column        int      `json:"column"`
	RequiredXP    int      `json:"required_xp"`
	Prerequisites []string `json:"prerequisites"`
	// Assunto do nó, neutro (`adhoc`, `graphs`): decide cor e ícone no app, que antes
	// adivinhava pelo nome. Vazio enquanto o conteúdo não preencher.
	Topic string `json:"topic"`
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

// GetSkillNodes devolve os nós publicados na língua, com o nome nela.
func (r *Repository) GetSkillNodes(ctx context.Context, locale string) ([]SkillNode, error) {
	query := publishedNodes + `
		SELECT n.id, nt.name, COALESCE(nt.description, ''), n.row_idx, n.col_idx, n.required_xp,
		       n.prerequisites, COALESCE(n.topic, '')
		FROM skill_nodes n
		JOIN publicados p ON p.id = n.id
		JOIN skill_node_translations nt ON nt.node_id = n.id AND nt.locale = $1
		ORDER BY n.row_idx ASC`
	rows, err := r.db.Query(ctx, query, locale)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	nodes := []SkillNode{}
	for rows.Next() {
		var n SkillNode
		var prereqsJSON []byte
		if err := rows.Scan(&n.ID, &n.Name, &n.Description, &n.Row, &n.Column, &n.RequiredXP, &prereqsJSON, &n.Topic); err != nil {
			return nil, err
		}
		json.Unmarshal(prereqsJSON, &n.Prerequisites)
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Pré-requisito que aponta para nó escondido nesta língua sai da lista: senão o nó
	// ficaria esperando um pai que o app nunca vai mostrar.
	published := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		published[n.ID] = true
	}
	for i := range nodes {
		kept := []string{}
		for _, p := range nodes[i].Prerequisites {
			if published[p] {
				kept = append(kept, p)
			}
		}
		nodes[i].Prerequisites = kept
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

// CreateUser cria a conta com a confirmação de idade, o país considerado nela (vazio
// grava NULL) e os aceites, numa transação só.
func (r *Repository) CreateUser(ctx context.Context, email, passwordHash string, ageConfirmed bool, country string, acceptances []LegalAcceptance) (string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	var id string
	query := `
		INSERT INTO users (email, password_hash, age_confirmed_at, country)
		VALUES ($1, $2, CASE WHEN $3::boolean THEN CURRENT_TIMESTAMP ELSE NULL END, NULLIF($4, ''))
		RETURNING id
	`
	err = tx.QueryRow(ctx, query, email, passwordHash, ageConfirmed, country).Scan(&id)
	if err != nil {
		return "", err
	}

	for _, acc := range acceptances {
		_, err = tx.Exec(ctx, `
			INSERT INTO legal_acceptances (user_id, kind, version, locale)
			VALUES ($1, $2, $3, $4)
		`, id, acc.Kind, acc.Version, acc.Locale)
		if err != nil {
			return "", err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}

	return id, nil
}

// ErrUserNotFound sinaliza e-mail sem conta.
var ErrUserNotFound = errors.New("user not found")

// ResetUserPassword troca a senha e derruba todas as sessões abertas, na mesma
// transação, e devolve o id do dono.
//
// Antes só trocava o hash. Quem tinha roubado a conta seguia com o refresh token, e a
// rotação o mantinha vivo para sempre: a troca de senha não expulsava ninguém.
//
// Também cancela uma exclusão pedida: provar que é dono pelo código do e-mail é o
// caminho de quem esqueceu a senha e quer a conta de volta. `restored` diz se havia
// exclusão a cancelar.
func (r *Repository) ResetUserPassword(ctx context.Context, email, passwordHash string) (userID string, restored bool, err error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback(ctx)

	// O valor antigo sai da CTE: o RETURNING do UPDATE já veria a coluna zerada.
	err = tx.QueryRow(ctx, `
		WITH old AS (SELECT id, deletion_requested_at FROM users WHERE email = $2 FOR UPDATE)
		UPDATE users u SET password_hash = $1, deletion_requested_at = NULL
		FROM old WHERE u.id = old.id
		RETURNING u.id, old.deletion_requested_at IS NOT NULL`,
		passwordHash, email).Scan(&userID, &restored)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, ErrUserNotFound
	}
	if err != nil {
		return "", false, err
	}

	if _, err := tx.Exec(ctx,
		`UPDATE refresh_tokens SET revoked = TRUE WHERE user_id = $1 AND revoked = FALSE`,
		userID); err != nil {
		return "", false, err
	}

	return userID, restored, tx.Commit(ctx)
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

// AccountDeletionGrace é a carência entre o pedido de exclusão e o expurgo. Entrar na
// conta ou redefinir a senha nesse prazo cancela a exclusão.
const AccountDeletionGrace = 30 * 24 * time.Hour

// MarkAccountForDeletion desativa a conta e derruba todas as sessões, numa transação,
// e devolve a partir de quando o expurgo pode apagá-la.
//
// Só gravava a data: o refresh token de outro aparelho seguia valendo, e a política
// promete encerrar as sessões em todos os aparelhos. Pedir de novo mantém a data do
// primeiro pedido, para ninguém empurrar o prazo sem querer.
func (r *Repository) MarkAccountForDeletion(ctx context.Context, userID string) (time.Time, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return time.Time{}, err
	}
	defer tx.Rollback(ctx)

	var requestedAt time.Time
	if err := tx.QueryRow(ctx, `
		UPDATE users SET deletion_requested_at = COALESCE(deletion_requested_at, CURRENT_TIMESTAMP)
		WHERE id = $1 RETURNING deletion_requested_at`, userID).Scan(&requestedAt); err != nil {
		return time.Time{}, err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE refresh_tokens SET revoked = TRUE WHERE user_id = $1 AND revoked = FALSE`,
		userID); err != nil {
		return time.Time{}, err
	}
	return requestedAt.Add(AccountDeletionGrace), tx.Commit(ctx)
}

// CancelAccountDeletion reativa a conta que tinha pedido exclusão e diz se havia o que
// cancelar. É o login dentro da carência.
func (r *Repository) CancelAccountDeletion(ctx context.Context, userID string) (bool, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE users SET deletion_requested_at = NULL
		WHERE id = $1 AND deletion_requested_at IS NOT NULL`, userID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (r *Repository) IsUserActive(ctx context.Context, userID string) bool {
	var deletionRequestedAt *time.Time
	err := r.db.QueryRow(ctx, "SELECT deletion_requested_at FROM users WHERE id = $1", userID).Scan(&deletionRequestedAt)
	if err != nil {
		return false
	}
	return deletionRequestedAt == nil
}

// PurgeDeletedAccounts apaga de vez as contas que passaram da carência e devolve
// quantas apagou.
//
// Uma transação por conta: antes era uma para todas, e uma conta com problema
// segurava o expurgo das outras. Os dados da conta nos dados de uso do PostHog não
// passam por aqui — somem pela retenção de 30 dias do projeto, e o app chama `reset()`
// ao excluir para não mandar mais nada com o id dela.
func (r *Repository) PurgeDeletedAccounts(ctx context.Context) (int, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id FROM users
		WHERE deletion_requested_at IS NOT NULL
		  AND deletion_requested_at < CURRENT_TIMESTAMP - make_interval(secs => $1)`,
		AccountDeletionGrace.Seconds())
	if err != nil {
		return 0, err
	}
	var userIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		userIDs = append(userIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	purged := 0
	for _, uid := range userIDs {
		ok, err := r.purgeAccount(ctx, uid)
		if err != nil {
			return purged, fmt.Errorf("expurgo de %s: %w", uid, err)
		}
		if ok {
			purged++
		}
	}
	return purged, nil
}

// purgeAccount apaga uma conta e tudo o que é dela, se ela ainda estiver vencida.
//
// A linha de `users` sai primeiro, conferindo de novo a carência: quem entrou na conta
// entre a seleção e este passo não perde nada. Em cascata vão `refresh_tokens`,
// `user_progress`, `user_paid_challenges` e `legal_acceptances`. `game_events` e
// `user_sync_state` não têm chave estrangeira para `users`, e `otps` é por e-mail:
// esses saem à mão.
func (r *Repository) purgeAccount(ctx context.Context, userID string) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	var email string
	err = tx.QueryRow(ctx, `
		DELETE FROM users
		WHERE id = $1 AND deletion_requested_at < CURRENT_TIMESTAMP - make_interval(secs => $2)
		RETURNING email`, userID, AccountDeletionGrace.Seconds()).Scan(&email)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	// game_events antes de user_sync_state: a chave estrangeira aponta para lá.
	for _, q := range []struct {
		sql string
		arg string
	}{
		{"DELETE FROM game_events WHERE user_id = $1", userID},
		{"DELETE FROM user_sync_state WHERE user_id = $1", userID},
		{"DELETE FROM otps WHERE email = $1", email},
	} {
		if _, err := tx.Exec(ctx, q.sql, q.arg); err != nil {
			return false, err
		}
	}
	return true, tx.Commit(ctx)
}

func (r *Repository) GetLatestLegalDocument(ctx context.Context, kind, locale string) (*LegalDocument, error) {
	query := `
		SELECT id, kind, locale, version, effective_at, material, body_html, created_at
		FROM legal_documents
		WHERE kind = $1 AND locale = $2
		ORDER BY version DESC LIMIT 1
	`
	var doc LegalDocument
	err := r.db.QueryRow(ctx, query, kind, locale).Scan(
		&doc.ID, &doc.Kind, &doc.Locale, &doc.Version, &doc.EffectiveAt, &doc.Material, &doc.BodyHTML, &doc.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func (r *Repository) GetLegalDocumentByVersion(ctx context.Context, kind string, version int) (*LegalDocument, error) {
	query := `
		SELECT id, kind, locale, version, effective_at, material, body_html, created_at
		FROM legal_documents
		WHERE kind = $1 AND version = $2
	`
	var doc LegalDocument
	err := r.db.QueryRow(ctx, query, kind, version).Scan(
		&doc.ID, &doc.Kind, &doc.Locale, &doc.Version, &doc.EffectiveAt, &doc.Material, &doc.BodyHTML, &doc.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func (r *Repository) GetPendingLegalDocuments(ctx context.Context, userID string) ([]LegalDocument, error) {
	// Pega a última versão de cada kind que seja <= current time,
	// mas que o usuário não aceitou uma versão >= ela.
	// Por simplificação (o PRD indica pegar pendentes relevantes).
	// "Uma versão com material: bloqueia o app... sem material: avisa"
	// Na verdade, vamos trazer os documentos recentes que o user não tem na tabela acceptances
	query := `
		WITH latest_docs AS (
			SELECT kind, MAX(version) as version
			FROM legal_documents
			GROUP BY kind
		)
		SELECT d.id, d.kind, d.locale, d.version, d.effective_at, d.material, d.body_html, d.created_at
		FROM legal_documents d
		JOIN latest_docs ld ON d.kind = ld.kind AND d.version = ld.version
		WHERE NOT EXISTS (
			SELECT 1 FROM legal_acceptances a
			WHERE a.user_id = $1 AND a.kind = d.kind AND a.version >= d.version
		)
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []LegalDocument
	for rows.Next() {
		var doc LegalDocument
		if err := rows.Scan(
			&doc.ID, &doc.Kind, &doc.Locale, &doc.Version, &doc.EffectiveAt, &doc.Material, &doc.BodyHTML, &doc.CreatedAt,
		); err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

func (r *Repository) AcceptLegalDocument(ctx context.Context, userID, kind string, version int, locale string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO legal_acceptances (user_id, kind, version, locale)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT DO NOTHING
	`, userID, kind, version, locale)
	return err
}
