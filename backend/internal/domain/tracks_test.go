package domain

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Trilha de manual, inventada: "Trilha T", com a amostra "Nó A" e o nó fechado "Nó B".
type paidTrack struct {
	id, productID      string
	sampleNode, closed string
	sampleCh, closedCh string
}

func testUUID(t *testing.T) string {
	t.Helper()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func seedPaidTrack(t *testing.T, conn *pgxpool.Pool) paidTrack {
	t.Helper()
	ctx := context.Background()
	suffix := testUUID(t)[:8]
	p := paidTrack{
		id:         testUUID(t),
		productID:  "com.example.logn.track." + suffix,
		sampleNode: testUUID(t),
		closed:     testUUID(t),
		sampleCh:   "test_paid_a_" + suffix,
		closedCh:   "test_paid_b_" + suffix,
	}
	payload := `{"content":{"code_lines":["int a;"]},"validation":{"type":"LINE_MATCH","correct_line":1}}`
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO tracks (id, slug, kind, author, app_store_product_id) VALUES ($1, $2, 'paid', 'Test', $3)`,
			[]any{p.id, "test-" + suffix, p.productID}},
		{`INSERT INTO track_translations (track_id, locale, name) VALUES ($1, 'pt-BR', 'Trilha T'), ($1, 'en', 'Track T'), ($1, 'es', 'Pista T')`,
			[]any{p.id}},
		{`INSERT INTO skill_nodes (id, track_id, row_idx, col_idx, required_xp, prerequisites) VALUES
		  ($1, $3, 0, 0, 0, '[]'), ($2, $3, 1, 0, 10, '[]')`, []any{p.sampleNode, p.closed, p.id}},
		{`INSERT INTO skill_node_translations (node_id, locale, name) VALUES
		  ($1, 'pt-BR', 'Nó A'), ($1, 'en', 'Node A'), ($1, 'es', 'Nodo A'),
		  ($2, 'pt-BR', 'Nó B'), ($2, 'en', 'Node B'), ($2, 'es', 'Nodo B')`, []any{p.sampleNode, p.closed}},
		{`INSERT INTO challenges (id, node_id, template_type, payload, position_idx) VALUES
		  ($1, $2, 'SPOT_THE_BUG', $5, 1), ($3, $4, 'SPOT_THE_BUG', $5, 1)`,
			[]any{p.sampleCh, p.sampleNode, p.closedCh, p.closed, payload}},
		{`INSERT INTO challenge_translations (challenge_id, locale, title, description, explanation) VALUES
		  ($1, 'pt-BR', 'Soma', 'd', 'e'), ($1, 'en', 'Sum', 'd', 'e'), ($1, 'es', 'Suma', 'd', 'e'),
		  ($2, 'pt-BR', 'Busca', 'd', 'e'), ($2, 'en', 'Search', 'd', 'e'), ($2, 'es', 'Busqueda', 'd', 'e')`,
			[]any{p.sampleCh, p.closedCh}},
	} {
		if _, err := conn.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatalf("semear: %v\n%s", err, q.sql)
		}
	}
	t.Cleanup(func() {
		conn.Exec(ctx, `DELETE FROM entitlements WHERE track_id = $1`, p.id)
		conn.Exec(ctx, `DELETE FROM entitlement_devices WHERE track_id = $1`, p.id)
		conn.Exec(ctx, `DELETE FROM store_transactions WHERE track_id = $1`, p.id)
		conn.Exec(ctx, `DELETE FROM track_keys WHERE track_id = $1`, p.id)
		conn.Exec(ctx, `DELETE FROM challenges WHERE id IN ($1, $2)`, p.sampleCh, p.closedCh)
		conn.Exec(ctx, `DELETE FROM skill_nodes WHERE id IN ($1, $2)`, p.sampleNode, p.closed)
		conn.Exec(ctx, `DELETE FROM tracks WHERE id = $1`, p.id)
	})
	return p
}

func seedUser(t *testing.T, conn *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()
	id := testUUID(t)
	if _, err := conn.Exec(ctx, `INSERT INTO users (id, email) VALUES ($1, $2)`, id, "paid-"+id[:8]+"@example.com"); err != nil {
		t.Fatalf("usuário: %v", err)
	}
	t.Cleanup(func() { conn.Exec(ctx, `DELETE FROM users WHERE id = $1`, id) })
	return id
}

func revokedCleanup(t *testing.T, conn *pgxpool.Pool, txID string) {
	t.Cleanup(func() {
		conn.Exec(context.Background(), `DELETE FROM revoked_transactions WHERE original_transaction_id = $1`, txID)
	})
}

func grant(p paidTrack, userID, token, txID string, restore bool) PurchaseGrant {
	return PurchaseGrant{
		UserID: userID, ProductID: p.productID, OriginalTransactionID: txID, TransactionID: txID,
		Environment: "Sandbox", AppAccountToken: token, RawPayload: "jws", Restore: restore,
	}
}

func useTestTrackKeySecret(t *testing.T) {
	old := TrackKeySecret
	TrackKeySecret = []byte("test-track-key-secret-32-bytes!!")
	t.Cleanup(func() { TrackKeySecret = old })
}

func TestPaidContentStaysOutOfTheOpenRoutes(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	p := seedPaidTrack(t, conn)
	ctx := context.Background()

	open, err := repo.GetChallenges(ctx, "pt-BR")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, c := range open {
		seen[c.ID] = true
	}
	if !seen[p.sampleCh] {
		t.Error("a amostra da trilha paga tem de sair aberta")
	}
	if seen[p.closedCh] {
		t.Error("desafio fechado da trilha paga saiu na rota aberta")
	}

	nodes, err := repo.GetSkillNodes(ctx, "pt-BR")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range nodes {
		switch n.ID {
		case p.sampleNode:
			if n.RequiresPurchase {
				t.Error("a amostra não pede compra")
			}
		case p.closed:
			if !n.RequiresPurchase {
				t.Error("o nó fechado pede compra")
			}
		}
	}

	closed, err := repo.GetTrackChallenges(ctx, p.id, "pt-BR")
	if err != nil {
		t.Fatal(err)
	}
	if len(closed) != 1 || closed[0].ID != p.closedCh {
		t.Errorf("o pacote leva só o fechado: %+v", closed)
	}
}

// O ataque: reembolsar, e reenviar o JWS antigo, que não sabe do reembolso. Nem na
// mesma conta, nem numa conta nova depois de excluir a antiga.
func TestARevokedTransactionNeverComesBack(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	p := seedPaidTrack(t, conn)
	ctx := context.Background()
	a, b := seedUser(t, conn), seedUser(t, conn)
	txID := "test-" + testUUID(t)[:12]
	revokedCleanup(t, conn, txID)

	if _, err := repo.GrantEntitlement(ctx, grant(p, a, a, txID, false)); err != nil {
		t.Fatalf("compra: %v", err)
	}
	if err := repo.RevokeTransaction(ctx, ProviderAppleStoreKit, txID, "refund", 1000); err != nil {
		t.Fatal(err)
	}
	if err := repo.CheckEntitlement(ctx, a, p.id); !errors.Is(err, ErrEntitlementRequired) {
		t.Fatalf("revogado continua com direito: %v", err)
	}

	if _, err := repo.GrantEntitlement(ctx, grant(p, a, a, txID, false)); !errors.Is(err, ErrTransactionRevoked) {
		t.Fatalf("reenvio na mesma conta: %v", err)
	}
	if _, err := repo.GrantEntitlement(ctx, grant(p, a, a, txID, true)); !errors.Is(err, ErrTransactionRevoked) {
		t.Fatalf("restauração na mesma conta: %v", err)
	}
	if _, err := conn.Exec(ctx, `DELETE FROM users WHERE id = $1`, a); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GrantEntitlement(ctx, grant(p, b, a, txID, true)); !errors.Is(err, ErrTransactionRevoked) {
		t.Fatalf("restauração em conta nova depois de excluir a antiga: %v", err)
	}
}

func TestRefundReversedGivesTheTrackBack(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	p := seedPaidTrack(t, conn)
	ctx := context.Background()
	a := seedUser(t, conn)
	txID := "test-" + testUUID(t)[:12]
	revokedCleanup(t, conn, txID)

	if _, err := repo.GrantEntitlement(ctx, grant(p, a, a, txID, false)); err != nil {
		t.Fatal(err)
	}
	if err := repo.RevokeTransaction(ctx, ProviderAppleStoreKit, txID, "refund", 1000); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReinstateRefund(ctx, ProviderAppleStoreKit, txID, 2000); err != nil {
		t.Fatal(err)
	}
	if err := repo.CheckEntitlement(ctx, a, p.id); err != nil {
		t.Fatalf("reembolso revertido devolve a trilha: %v", err)
	}
	if _, err := repo.GrantEntitlement(ctx, grant(p, a, a, txID, false)); err != nil {
		t.Fatalf("transação reinstaurada volta a valer: %v", err)
	}

	// O REFUND original, reenviado pela Apple depois da reversão, não revoga de novo.
	if err := repo.RevokeTransaction(ctx, ProviderAppleStoreKit, txID, "refund", 1000); err != nil {
		t.Fatal(err)
	}
	if err := repo.CheckEntitlement(ctx, a, p.id); err != nil {
		t.Fatalf("REFUND atrasado revogou de novo: %v", err)
	}
	// Um reembolso novo, depois da reversão, revoga.
	if err := repo.RevokeTransaction(ctx, ProviderAppleStoreKit, txID, "refund", 3000); err != nil {
		t.Fatal(err)
	}
	if err := repo.CheckEntitlement(ctx, a, p.id); !errors.Is(err, ErrEntitlementRequired) {
		t.Fatalf("reembolso novo não revogou: %v", err)
	}
}

// A reversão chegou antes do reembolso que ela reverte: o reembolso, mais velho, não vale.
func TestARefundOlderThanItsReversalIsIgnored(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	p := seedPaidTrack(t, conn)
	ctx := context.Background()
	a := seedUser(t, conn)
	txID := "test-" + testUUID(t)[:12]
	revokedCleanup(t, conn, txID)

	if _, err := repo.GrantEntitlement(ctx, grant(p, a, a, txID, false)); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReinstateRefund(ctx, ProviderAppleStoreKit, txID, 2000); err != nil {
		t.Fatal(err)
	}
	if err := repo.RevokeTransaction(ctx, ProviderAppleStoreKit, txID, "refund", 1000); err != nil {
		t.Fatal(err)
	}
	if err := repo.CheckEntitlement(ctx, a, p.id); err != nil {
		t.Fatalf("reembolso já revertido revogou: %v", err)
	}
}

// Conceder e revogar a mesma transação ao mesmo tempo nunca termina com direito ativo
// de uma compra reembolsada.
func TestGrantAndRevokeDoNotRace(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	p := seedPaidTrack(t, conn)
	ctx := context.Background()

	for i := 0; i < 20; i++ {
		a := seedUser(t, conn)
		txID := "test-" + testUUID(t)[:12]
		revokedCleanup(t, conn, txID)
		done := make(chan struct{})
		go func() {
			repo.GrantEntitlement(ctx, grant(p, a, a, txID, false))
			close(done)
		}()
		if err := repo.RevokeTransaction(ctx, ProviderAppleStoreKit, txID, "refund", 1000); err != nil {
			t.Fatal(err)
		}
		<-done
		if err := repo.CheckEntitlement(ctx, a, p.id); !errors.Is(err, ErrEntitlementRequired) {
			t.Fatalf("rodada %d: compra reembolsada ficou ativa", i)
		}
	}
}

// Revogação manual não sai por REFUND_REVERSED.
func TestAManualRevocationIsNotUndoneByARefundReversal(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	p := seedPaidTrack(t, conn)
	ctx := context.Background()
	a := seedUser(t, conn)
	txID := "test-" + testUUID(t)[:12]
	revokedCleanup(t, conn, txID)

	if _, err := repo.GrantEntitlement(ctx, grant(p, a, a, txID, false)); err != nil {
		t.Fatal(err)
	}
	if err := repo.RevokeTransaction(ctx, ProviderAppleStoreKit, txID, "redistribution", 1000); err != nil {
		t.Fatal(err)
	}
	// Um REFUND em cima da manual não a torna reversível.
	if err := repo.RevokeTransaction(ctx, ProviderAppleStoreKit, txID, "refund", 1500); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReinstateRefund(ctx, ProviderAppleStoreKit, txID, 2000); err != nil {
		t.Fatal(err)
	}
	if err := repo.CheckEntitlement(ctx, a, p.id); !errors.Is(err, ErrEntitlementRequired) {
		t.Fatalf("revogação manual desfeita: %v", err)
	}
}

func TestOneActiveAccountPerTransaction(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	p := seedPaidTrack(t, conn)
	ctx := context.Background()
	a, b := seedUser(t, conn), seedUser(t, conn)
	txID := "test-" + testUUID(t)[:12]

	if _, err := repo.GrantEntitlement(ctx, grant(p, a, a, txID, false)); err != nil {
		t.Fatal(err)
	}
	// Reenviar a própria compra é idempotente, e a auditoria não duplica.
	if _, err := repo.GrantEntitlement(ctx, grant(p, a, a, txID, false)); err != nil {
		t.Fatalf("reenvio da própria compra: %v", err)
	}
	var rows int
	conn.QueryRow(ctx, `SELECT count(*) FROM store_transactions WHERE provider_transaction_id = $1`, txID).Scan(&rows)
	if rows != 1 {
		t.Fatalf("store_transactions com %d linhas para uma transação", rows)
	}

	if _, err := repo.GrantEntitlement(ctx, grant(p, b, a, txID, false)); !errors.Is(err, ErrAccountMismatch) {
		t.Fatalf("compra com o token de outra conta: %v", err)
	}
	if _, err := repo.GrantEntitlement(ctx, grant(p, b, a, txID, true)); !errors.Is(err, ErrOwnedByOtherAccount) {
		t.Fatalf("restauração da compra de outra conta ativa: %v", err)
	}
	if _, err := repo.GrantEntitlement(ctx, grant(p, b, "", txID, true)); !errors.Is(err, ErrOwnedByOtherAccount) {
		t.Fatalf("restauração sem token de compra que outra conta ativa tem: %v", err)
	}

	// Exclusão pedida ainda segura a compra: ela se desfaz com uma troca de senha, e
	// soltar ali deixava a compra rodar entre contas.
	if _, err := conn.Exec(ctx, `UPDATE users SET deletion_requested_at = CURRENT_TIMESTAMP WHERE id = $1`, a); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GrantEntitlement(ctx, grant(p, b, a, txID, true)); !errors.Is(err, ErrOwnedByOtherAccount) {
		t.Fatalf("restauração com a dona só com exclusão pedida: %v", err)
	}

	// Depois do expurgo, a compra volta numa conta nova (spec, critério 6).
	if _, err := conn.Exec(ctx, `DELETE FROM users WHERE id = $1`, a); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GrantEntitlement(ctx, grant(p, b, a, txID, true)); err != nil {
		t.Fatalf("restauração depois do expurgo da dona: %v", err)
	}
	if err := repo.CheckEntitlement(ctx, b, p.id); err != nil {
		t.Fatalf("conta nova sem a trilha: %v", err)
	}
}

func TestAnUnknownProductGrantsNothing(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	p := seedPaidTrack(t, conn)
	a := seedUser(t, conn)
	g := grant(p, a, a, "test-"+testUUID(t)[:12], false)
	g.ProductID = "com.example.other"
	if _, err := NewRepository(conn).GrantEntitlement(context.Background(), g); !errors.Is(err, ErrUnknownProduct) {
		t.Fatalf("produto desconhecido: %v", err)
	}
}

func answer(t *testing.T, challengeID, nodeID string) GameEvent {
	return GameEvent{
		ID:          testUUID(t),
		EventType:   "MATCH_ANSWER",
		PayloadJSON: fmt.Sprintf(`{"is_correct":true,"challenge_id":%q,"node_id":%q,"template_type":"SPOT_THE_BUG"}`, challengeID, nodeID),
	}
}

func processAnswer(t *testing.T, repo *Repository, conn *pgxpool.Pool, userID string, e GameEvent) {
	t.Helper()
	ctx := context.Background()
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := repo.ProcessEventXP(ctx, tx, userID, e); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func xpOf(t *testing.T, conn *pgxpool.Pool, userID string) int {
	var xp int
	if err := conn.QueryRow(context.Background(), `SELECT global_xp FROM users WHERE id = $1`, userID).Scan(&xp); err != nil {
		t.Fatal(err)
	}
	return xp
}

// Evento de trilha sem direito ativo entra na cadeia e não paga (spec, critério 8); a
// amostra paga para todos; id que o banco não conhece não paga.
func TestTheSyncPaysAPaidTrackOnlyWithEntitlement(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	p := seedPaidTrack(t, conn)
	ctx := context.Background()
	a := seedUser(t, conn)

	processAnswer(t, repo, conn, a, answer(t, p.closedCh, p.closed))
	if xp := xpOf(t, conn, a); xp != 0 {
		t.Fatalf("fechado sem compra pagou %d", xp)
	}
	processAnswer(t, repo, conn, a, answer(t, "test_invented_id", p.sampleNode))
	if xp := xpOf(t, conn, a); xp != 0 {
		t.Fatalf("desafio inventado pagou %d", xp)
	}
	// O nó do evento é ignorado: vale o do banco.
	processAnswer(t, repo, conn, a, answer(t, p.sampleCh, p.closed))
	if xp := xpOf(t, conn, a); xp != XPPerAcceptedAnswer {
		t.Fatalf("amostra pagou %d", xp)
	}

	if _, err := repo.GrantEntitlement(ctx, grant(p, a, a, "test-"+testUUID(t)[:12], false)); err != nil {
		t.Fatal(err)
	}
	processAnswer(t, repo, conn, a, answer(t, p.closedCh, p.closed))
	if xp := xpOf(t, conn, a); xp != 2*XPPerAcceptedAnswer {
		t.Fatalf("fechado com compra: %d", xp)
	}

	stats, err := repo.GetUserStats(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if stats.PaidTrackXP[p.id] != 2*XPPerAcceptedAnswer {
		t.Fatalf("XP da trilha: %v", stats.PaidTrackXP)
	}
	var sampleProgress int
	conn.QueryRow(ctx, `SELECT current_xp FROM user_progress WHERE user_id = $1 AND node_id = $2`, a, p.sampleNode).Scan(&sampleProgress)
	if sampleProgress != XPPerAcceptedAnswer {
		t.Fatalf("progresso foi para o nó do evento, não o do desafio: %d", sampleProgress)
	}
}

func TestTheLicenseOpensThePackage(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	useTestTrackKeySecret(t)
	repo := NewRepository(conn)
	p := seedPaidTrack(t, conn)
	ctx := context.Background()
	a := seedUser(t, conn)
	now := time.Unix(1_800_000_000, 0)

	if _, err := repo.IssueLicense(ctx, a, p.id, testUUID(t), now); !errors.Is(err, ErrEntitlementRequired) {
		t.Fatalf("licença sem compra: %v", err)
	}
	if _, _, err := repo.BuildTrackPackage(ctx, a, p.id); !errors.Is(err, ErrEntitlementRequired) {
		t.Fatalf("pacote sem compra: %v", err)
	}
	if _, err := repo.GrantEntitlement(ctx, grant(p, a, a, "test-"+testUUID(t)[:12], false)); err != nil {
		t.Fatal(err)
	}

	// Sem limite de aparelhos: cada um fica registrado, nenhum é barrado.
	var license *License
	for i := 0; i < 5; i++ {
		l, err := repo.IssueLicense(ctx, a, p.id, testUUID(t), now)
		if err != nil {
			t.Fatalf("aparelho %d: %v", i+1, err)
		}
		license = l
	}
	var devices int
	conn.QueryRow(ctx, `SELECT count(*) FROM entitlement_devices WHERE user_id = $1 AND track_id = $2`, a, p.id).Scan(&devices)
	if devices != 5 {
		t.Fatalf("aparelhos registrados: %d", devices)
	}
	if license.ValidUntil-license.IssuedAt != int64(OfflineLicenseValidity/time.Second) || len(license.KeyHex) != 64 {
		t.Fatalf("licença: %+v", license)
	}

	// A chave no banco está cifrada, não em claro.
	key, _ := hex.DecodeString(license.KeyHex)
	var wrapped []byte
	conn.QueryRow(ctx, `SELECT wrapped_key FROM track_keys WHERE track_id = $1`, p.id).Scan(&wrapped)
	if len(wrapped) != 60 || bytes.Contains(wrapped, key) {
		t.Fatalf("track_keys guarda a chave em claro: %x", wrapped)
	}

	blob, version, err := repo.BuildTrackPackage(ctx, a, p.id)
	if err != nil {
		t.Fatal(err)
	}
	content, err := OpenTrackPackage(license.KeyHex, p.id, version, blob)
	if err != nil {
		t.Fatalf("pacote não abre com a licença: %v", err)
	}
	pt := content.Challenges["pt-BR"]
	if len(pt) != 1 || pt[0].ID != p.closedCh || len(content.Challenges["es"]) != 1 {
		t.Fatalf("conteúdo do pacote: %+v", content.Challenges)
	}
	if _, err := OpenTrackPackage(license.KeyHex, p.id, version+1, blob); err == nil {
		t.Fatal("pacote abriu como se fosse de outra versão")
	}

	// Sem o segredo certo, a chave guardada não abre.
	TrackKeySecret = []byte("another-secret-that-is-32-bytes!")
	if _, err := repo.IssueLicense(ctx, a, p.id, testUUID(t), now); err == nil {
		t.Fatal("chave guardada abriu com outro segredo")
	}
}
