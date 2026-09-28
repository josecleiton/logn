package domain

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/josecleiton/logn/backend/internal/locale"
)

// TrackKeySecret cifra as chaves de conteúdo guardadas em `track_keys`. Vem de
// TRACK_KEY_SECRET; o servidor não sobe em produção sem ela, como com JWT_SECRET.
var TrackKeySecret []byte

// ProviderAppleStoreKit é o provedor das compras da App Store, como vai no banco.
const ProviderAppleStoreKit = "apple_storekit"

// OfflineLicenseValidity é quanto a trilha comprada abre sem falar com o servidor.
const OfflineLicenseValidity = 30 * 24 * time.Hour

var (
	ErrEntitlementRequired = errors.New("no active entitlement for this track")
	ErrUnknownProduct      = errors.New("product is not a paid track")
	ErrTransactionRevoked  = errors.New("transaction was revoked")
	ErrOwnedByOtherAccount = errors.New("transaction belongs to another active account")
	ErrAccountMismatch     = errors.New("transaction was bought by another account")
)

// Track é uma trilha do catálogo, na língua pedida: a principal e as pagas.
type Track struct {
	ID     string `json:"id"`
	Slug   string `json:"slug"`
	Kind   string `json:"kind"`
	Status string `json:"status"`
	Author string `json:"author"`
	// Vazio na gratuita.
	ProductID      string `json:"product_id"`
	ContentVersion int    `json:"content_version"`
	// A cor do balão da trilha, `#RRGGBB`.
	Color        string `json:"color"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	NodeCount    int    `json:"node_count"`
	ProblemCount int    `json:"problem_count"`
	// Línguas em que a trilha foi publicada.
	Languages []string `json:"languages"`
	// A conta que pediu tem direito ativo. Falso sem conta.
	Owned bool `json:"owned"`
	// Motivo, quando o direito da conta foi revogado; vazio no resto.
	RevokedReason string `json:"revoked_reason"`
}

// GetTracks devolve o catálogo, a principal primeiro. A descontinuada sai para quem não
// comprou e fica para quem comprou (spec, seção 2); a indisponível só aparece para quem
// a conta libera (`visibleTrack`). Trilha sem nome na língua não aparece, como o nó.
func (r *Repository) GetTracks(ctx context.Context, lang, userID string) ([]Track, error) {
	rows, err := r.db.Query(ctx, `
		WITH mine AS (
			SELECT track_id, status, revoked_reason FROM entitlements WHERE user_id = NULLIF($2, '')::uuid)
		SELECT t.id, t.slug, t.kind, t.status, t.author, COALESCE(t.app_store_product_id, ''),
		       t.content_version, t.color, tt.name, COALESCE(tt.description, ''),
		       (SELECT count(*) FROM skill_nodes n WHERE n.track_id = t.id),
		       (SELECT count(*) FROM challenges c JOIN skill_nodes n ON n.id = c.node_id WHERE n.track_id = t.id),
		       ARRAY(SELECT x.locale FROM track_translations x WHERE x.track_id = t.id ORDER BY x.locale),
		       EXISTS (SELECT 1 FROM mine WHERE mine.track_id = t.id AND mine.status = 'active'),
		       COALESCE((SELECT mine.revoked_reason FROM mine WHERE mine.track_id = t.id AND mine.status = 'revoked'), '')
		FROM tracks t
		JOIN track_translations tt ON tt.track_id = t.id AND tt.locale = $1
		WHERE (t.status = 'active' AND `+visibleTrack("$2")+`)
		   OR EXISTS (SELECT 1 FROM mine WHERE mine.track_id = t.id AND mine.status = 'active')
		ORDER BY t.kind = 'paid', t.slug`, lang, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tracks := []Track{}
	for rows.Next() {
		var t Track
		if err := rows.Scan(&t.ID, &t.Slug, &t.Kind, &t.Status, &t.Author, &t.ProductID, &t.ContentVersion,
			&t.Color, &t.Name, &t.Description, &t.NodeCount, &t.ProblemCount, &t.Languages,
			&t.Owned, &t.RevokedReason); err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}

// PurchaseGrant é uma transação já verificada, pronta para virar direito de acesso.
type PurchaseGrant struct {
	UserID                string
	ProductID             string
	OriginalTransactionID string
	TransactionID         string
	Environment           string
	AppAccountToken       string
	RawPayload            string
	// Restauração: a transação pode ter sido comprada por outra conta, desde que ela
	// não esteja mais ativa (spec, seção 5).
	Restore bool
}

// GrantEntitlement liga a transação à conta e devolve a trilha liberada.
//
// Uma transação revogada não volta por aqui, nem nesta conta nem em outra: o registro
// de revogadas não some com a conta. Antes o upsert reativava o direito de quem
// reenviava o JWS de uma compra já reembolsada.
func (r *Repository) GrantEntitlement(ctx context.Context, g PurchaseGrant) (string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	if err := lockTransaction(ctx, tx, ProviderAppleStoreKit, g.OriginalTransactionID); err != nil {
		return "", err
	}

	var trackID string
	err = tx.QueryRow(ctx, `SELECT id FROM tracks WHERE app_store_product_id = $1 AND kind = 'paid'`, g.ProductID).Scan(&trackID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrUnknownProduct
	}
	if err != nil {
		return "", err
	}

	var revoked bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM revoked_transactions
		               WHERE provider = $1 AND original_transaction_id = $2 AND reversed_at_ms IS NULL)`,
		ProviderAppleStoreKit, g.OriginalTransactionID).Scan(&revoked); err != nil {
		return "", err
	}
	if revoked {
		return "", ErrTransactionRevoked
	}

	// A compra nasce com o id da conta no `appAccountToken`, posto pelo app. Na compra
	// ele tem de ser o de quem manda; na restauração pode ser de outra conta só se ela
	// não existe mais.
	//
	// Conta com exclusão pedida ainda segura a compra: a exclusão se desfaz com uma
	// troca de senha, e soltar a compra ali deixava uma compra só rodar entre contas —
	// pede exclusão, o próximo restaura, desfaz, repete. A compra volta numa conta nova
	// depois do expurgo (spec, critério 6).
	sameAccount := strings.EqualFold(g.AppAccountToken, g.UserID)
	if !g.Restore && !sameAccount {
		return "", ErrAccountMismatch
	}
	if g.Restore && g.AppAccountToken != "" && !sameAccount {
		var otherExists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE id::text = lower($1))`,
			g.AppAccountToken).Scan(&otherExists); err != nil {
			return "", err
		}
		if otherExists {
			return "", ErrOwnedByOtherAccount
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO store_transactions
			(user_id, track_id, provider, provider_transaction_id, original_transaction_id, environment, raw_payload)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (provider, provider_transaction_id) DO NOTHING`,
		g.UserID, trackID, ProviderAppleStoreKit, g.TransactionID, g.OriginalTransactionID, g.Environment, g.RawPayload); err != nil {
		return "", err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO entitlements (user_id, track_id, original_transaction_id, status)
		VALUES ($1, $2, $3, 'active')
		ON CONFLICT (user_id, track_id) DO UPDATE
		SET status = 'active', revoked_reason = NULL, original_transaction_id = EXCLUDED.original_transaction_id`,
		g.UserID, trackID, g.OriginalTransactionID)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "entitlements_one_active_owner" {
		return "", ErrOwnedByOtherAccount
	}
	if err != nil {
		return "", err
	}
	return trackID, tx.Commit(ctx)
}

// lockTransaction serializa, até o fim da transação do banco, tudo que mexe numa mesma
// transação da loja. Sem isto, uma restauração lia "não revogada", o REFUND gravava a
// revogação sem ver a linha ainda não confirmada da restauração, e a restauração
// confirmava um direito ativo para uma compra reembolsada.
func lockTransaction(ctx context.Context, tx pgx.Tx, provider, originalTransactionID string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`,
		provider+":"+originalTransactionID)
	return err
}

// RevokeTransaction revoga a transação: grava entre as revogadas e tira o direito de
// quem a tiver. Serve à notificação da Apple e ao runbook de revogação manual, com
// `signedAtMs` a hora da notificação (ou de agora, na manual).
//
// Revogação que já vale não muda de motivo: um REFUND em cima de uma revogação manual
// não a torna reversível. Reembolso já revertido só volta com um REFUND mais novo que a
// reversão — o atrasado, que a Apple reenviou, não conta.
func (r *Repository) RevokeTransaction(ctx context.Context, provider, originalTransactionID, reason string, signedAtMs int64) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := lockTransaction(ctx, tx, provider, originalTransactionID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO revoked_transactions (provider, original_transaction_id, reason, revoked_at_ms)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (provider, original_transaction_id) DO UPDATE
		SET reason = EXCLUDED.reason, revoked_at_ms = EXCLUDED.revoked_at_ms, reversed_at_ms = NULL
		WHERE revoked_transactions.reversed_at_ms IS NOT NULL
		  AND revoked_transactions.reversed_at_ms < EXCLUDED.revoked_at_ms`,
		provider, originalTransactionID, reason, signedAtMs); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE entitlements e SET status = 'revoked', revoked_reason = rt.reason
		FROM revoked_transactions rt
		WHERE rt.provider = $1 AND rt.original_transaction_id = $2 AND rt.reversed_at_ms IS NULL
		  AND e.original_transaction_id = rt.original_transaction_id AND e.status = 'active'`,
		provider, originalTransactionID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ReinstateRefund desfaz a revogação de um reembolso que a Apple reverteu, se a
// reversão for mais nova que o reembolso. Revogação manual e da loja não saem por aqui.
// Reversão de reembolso que ainda não chegou fica gravada, para o REFUND atrasado que
// vier depois ser ignorado.
func (r *Repository) ReinstateRefund(ctx context.Context, provider, originalTransactionID string, signedAtMs int64) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := lockTransaction(ctx, tx, provider, originalTransactionID); err != nil {
		return err
	}

	tag, err := tx.Exec(ctx, `
		INSERT INTO revoked_transactions (provider, original_transaction_id, reason, revoked_at_ms, reversed_at_ms)
		VALUES ($1, $2, 'refund', 0, $3)
		ON CONFLICT (provider, original_transaction_id) DO UPDATE
		SET reversed_at_ms = EXCLUDED.reversed_at_ms
		WHERE revoked_transactions.reason = 'refund'
		  AND revoked_transactions.reversed_at_ms IS NULL
		  AND revoked_transactions.revoked_at_ms < EXCLUDED.reversed_at_ms`,
		provider, originalTransactionID, signedAtMs)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, `
		UPDATE entitlements SET status = 'active', revoked_reason = NULL
		WHERE original_transaction_id = $1 AND revoked_reason = 'refund'`,
		originalTransactionID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// CheckEntitlement confere se a conta tem direito ativo à trilha.
func (r *Repository) CheckEntitlement(ctx context.Context, userID, trackID string) error {
	var active bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM entitlements WHERE user_id = $1 AND track_id = $2 AND status = 'active')`,
		userID, trackID).Scan(&active)
	if err != nil {
		return err
	}
	if !active {
		return ErrEntitlementRequired
	}
	return nil
}

// License é o que o aparelho guarda para abrir a trilha sem rede.
type License struct {
	TrackID        string `json:"track_id"`
	KeyHex         string `json:"key_hex"`
	ContentVersion int    `json:"content_version"`
	IssuedAt       int64  `json:"issued_at"`
	ValidUntil     int64  `json:"valid_until"`
}

// IssueLicense registra o aparelho e devolve a chave da versão atual da trilha.
//
// O registro não tem limite (spec, seção 2): ele é evidência para a revogação por
// compartilhamento, que olha aparelhos ativos ao mesmo tempo, e não uma trava.
func (r *Repository) IssueLicense(ctx context.Context, userID, trackID, deviceID string, now time.Time) (*License, error) {
	if err := r.CheckEntitlement(ctx, userID, trackID); err != nil {
		return nil, err
	}
	if _, err := r.db.Exec(ctx, `
		INSERT INTO entitlement_devices (user_id, track_id, device_id, last_seen_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
		ON CONFLICT (user_id, track_id, device_id) DO UPDATE SET last_seen_at = CURRENT_TIMESTAMP`,
		userID, trackID, deviceID); err != nil {
		return nil, err
	}

	version, err := r.contentVersion(ctx, trackID)
	if err != nil {
		return nil, err
	}
	key, err := r.trackKey(ctx, trackID, version)
	if err != nil {
		return nil, err
	}
	return &License{
		TrackID:        trackID,
		KeyHex:         hex.EncodeToString(key),
		ContentVersion: version,
		IssuedAt:       now.Unix(),
		ValidUntil:     now.Add(OfflineLicenseValidity).Unix(),
	}, nil
}

// TrackPackageContent é o conteúdo fechado da trilha, nas línguas publicadas, antes
// da cifra. Os desafios têm a forma de `GET /api/v1/challenges`.
type TrackPackageContent struct {
	TrackID        string                 `json:"track_id"`
	ContentVersion int                    `json:"content_version"`
	Challenges     map[string][]Challenge `json:"challenges"`
}

// BuildTrackPackage devolve o pacote cifrado da trilha e a versão dele.
//
// Formato: nonce de 12 bytes seguido do AES-256-GCM do JSON, com a chave de conteúdo da
// versão e `PackageAAD` como dado associado — um pacote de outra trilha ou de outra
// versão não abre com esta licença.
func (r *Repository) BuildTrackPackage(ctx context.Context, userID, trackID string) ([]byte, int, error) {
	if err := r.CheckEntitlement(ctx, userID, trackID); err != nil {
		return nil, 0, err
	}
	version, err := r.contentVersion(ctx, trackID)
	if err != nil {
		return nil, 0, err
	}

	content := TrackPackageContent{TrackID: trackID, ContentVersion: version, Challenges: map[string][]Challenge{}}
	for _, lang := range locale.Supported {
		challenges, err := r.GetTrackChallenges(ctx, trackID, lang)
		if err != nil {
			return nil, 0, err
		}
		content.Challenges[lang] = challenges
	}
	plain, err := json.Marshal(content)
	if err != nil {
		return nil, 0, err
	}

	key, err := r.trackKey(ctx, trackID, version)
	if err != nil {
		return nil, 0, err
	}
	blob, err := seal(key, plain, PackageAAD(trackID, version))
	if err != nil {
		return nil, 0, err
	}
	return blob, version, nil
}

// PackageAAD é o dado associado da cifra do pacote. O Core monta o mesmo.
func PackageAAD(trackID string, version int) []byte {
	return []byte("logn-track:" + trackID + ":" + strconv.Itoa(version))
}

// OpenTrackPackage abre um pacote. Existe para o teste provar o formato.
func OpenTrackPackage(keyHex string, trackID string, version int, blob []byte) (*TrackPackageContent, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, err
	}
	plain, err := open(key, blob, PackageAAD(trackID, version))
	if err != nil {
		return nil, err
	}
	var content TrackPackageContent
	if err := json.Unmarshal(plain, &content); err != nil {
		return nil, err
	}
	return &content, nil
}

func (r *Repository) contentVersion(ctx context.Context, trackID string) (int, error) {
	var version int
	err := r.db.QueryRow(ctx, `SELECT content_version FROM tracks WHERE id = $1 AND kind = 'paid'`, trackID).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrEntitlementRequired
	}
	return version, err
}

// trackKey devolve a chave de conteúdo da versão, e cria na primeira vez. Duas
// criações ao mesmo tempo terminam com a mesma chave: vale a que entrou primeiro.
//
// A versão anterior devolvia uma chave fixa de zeros quando faltava a linha.
func (r *Repository) trackKey(ctx context.Context, trackID string, version int) ([]byte, error) {
	secret, err := trackKeySecret()
	if err != nil {
		return nil, err
	}
	aad := []byte("logn-track-key:" + trackID + ":" + strconv.Itoa(version))

	var wrapped []byte
	err = r.db.QueryRow(ctx, `SELECT wrapped_key FROM track_keys WHERE track_id = $1 AND content_version = $2`, trackID, version).Scan(&wrapped)
	if errors.Is(err, pgx.ErrNoRows) {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		fresh, sealErr := seal(secret, key, aad)
		if sealErr != nil {
			return nil, sealErr
		}
		if _, err := r.db.Exec(ctx, `
			INSERT INTO track_keys (track_id, content_version, wrapped_key) VALUES ($1, $2, $3)
			ON CONFLICT (track_id, content_version) DO NOTHING`, trackID, version, fresh); err != nil {
			return nil, err
		}
		err = r.db.QueryRow(ctx, `SELECT wrapped_key FROM track_keys WHERE track_id = $1 AND content_version = $2`, trackID, version).Scan(&wrapped)
	}
	if err != nil {
		return nil, err
	}
	key, err := open(secret, wrapped, aad)
	if err != nil {
		return nil, fmt.Errorf("track key for %s v%d does not open with TRACK_KEY_SECRET: %w", trackID, version, err)
	}
	return key, nil
}

func trackKeySecret() ([]byte, error) {
	if len(TrackKeySecret) != 32 {
		return nil, errors.New("TRACK_KEY_SECRET must be 32 bytes")
	}
	return TrackKeySecret, nil
}

func seal(key, plain, aad []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plain, aad), nil
}

func open(key, blob, aad []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(blob) < gcm.NonceSize() {
		return nil, errors.New("sealed blob too short")
	}
	return gcm.Open(nil, blob[:gcm.NonceSize()], blob[gcm.NonceSize():], aad)
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
