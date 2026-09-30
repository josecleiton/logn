package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/josecleiton/logn/backend/internal/domain"
	"github.com/josecleiton/logn/backend/internal/storekit"
	"github.com/josecleiton/logn/backend/internal/storekit/storekittest"
)

const testBundle = "com.example.logn"

func newTestUUID(t *testing.T) string {
	t.Helper()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// storeFixture é uma trilha paga inventada, dois usuários e a cadeia que assina as
// compras. O servidor confia só nessa cadeia.
type storeFixture struct {
	server    *Server
	conn      *pgxpool.Pool
	chain     *storekittest.Chain
	trackID   string
	productID string
	alice     string
	bob       string
}

func newStoreFixture(t *testing.T) *storeFixture {
	t.Helper()
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	ctx := context.Background()

	old := domain.TrackKeySecret
	domain.TrackKeySecret = []byte("test-track-key-secret-32-bytes!!")
	t.Cleanup(func() { domain.TrackKeySecret = old })

	chain := storekittest.NewChain(t, storekittest.Options{})
	v, err := storekit.NewValidator(storekit.Config{
		BundleID:     testBundle,
		Environments: []string{storekit.EnvProduction, storekit.EnvSandbox},
		Roots:        chain.Roots(),
	})
	if err != nil {
		t.Fatal(err)
	}

	f := &storeFixture{
		server:    &Server{repo: domain.NewRepository(conn), storekit: v},
		conn:      conn,
		chain:     chain,
		trackID:   newTestUUID(t),
		productID: "com.example.logn.track." + newTestUUID(t)[:8],
		alice:     newTestUUID(t),
		bob:       newTestUUID(t),
	}
	if _, err := conn.Exec(ctx, `INSERT INTO tracks (id, slug, kind, author, store_product_id) VALUES ($1, $2, 'paid', 'Test', $3)`,
		f.trackID, "test-"+f.trackID[:8], f.productID); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO track_translations (track_id, locale, name) VALUES ($1, 'pt-BR', 'Trilha T')`, f.trackID); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{f.alice, f.bob} {
		if _, err := conn.Exec(ctx, `INSERT INTO users (id, email) VALUES ($1, $2)`, id, "store-"+id[:8]+"@example.com"); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		conn.Exec(ctx, `DELETE FROM users WHERE id IN ($1, $2)`, f.alice, f.bob)
		conn.Exec(ctx, `DELETE FROM entitlements WHERE track_id = $1`, f.trackID)
		conn.Exec(ctx, `DELETE FROM entitlement_devices WHERE track_id = $1`, f.trackID)
		conn.Exec(ctx, `DELETE FROM revoked_transactions WHERE original_transaction_id IN (SELECT original_transaction_id FROM store_transactions WHERE track_id = $1)`, f.trackID)
		conn.Exec(ctx, `DELETE FROM manual_revocations WHERE original_transaction_id IN (SELECT original_transaction_id FROM store_transactions WHERE track_id = $1)`, f.trackID)
		conn.Exec(ctx, `DELETE FROM store_transactions WHERE track_id = $1`, f.trackID)
		conn.Exec(ctx, `DELETE FROM track_keys WHERE track_id = $1`, f.trackID)
		conn.Exec(ctx, `DELETE FROM tracks WHERE id = $1`, f.trackID)
	})
	return f
}

func (f *storeFixture) purchaseJWS(t *testing.T, txID, accountToken string) string {
	return f.chain.Sign(t, storekittest.Transaction(testBundle, f.productID, txID, accountToken))
}

func (f *storeFixture) post(t *testing.T, handler http.HandlerFunc, path, userID string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	if userID != "" {
		req.Header.Set("Authorization", bearer(t, userID))
	}
	rr := httptest.NewRecorder()
	handler(rr, req)
	return rr
}

func (f *storeFixture) license(t *testing.T, userID, deviceID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tracks/"+f.trackID+"/license", nil)
	req.SetPathValue("id", f.trackID)
	req.Header.Set("Authorization", bearer(t, userID))
	if deviceID != "" {
		req.Header.Set("X-Device-ID", deviceID)
	}
	rr := httptest.NewRecorder()
	f.server.trackLicenseHandler(rr, req)
	return rr
}

func errorCode(t *testing.T, rr *httptest.ResponseRecorder) string {
	t.Helper()
	var body apiError
	json.Unmarshal(rr.Body.Bytes(), &body)
	return body.Code
}

func expect(t *testing.T, rr *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rr.Code != status || (code != "" && errorCode(t, rr) != code) {
		t.Fatalf("got %d %s, want %d %s", rr.Code, rr.Body.String(), status, code)
	}
}

func TestAPurchaseNeedsTheAppleChain(t *testing.T) {
	f := newStoreFixture(t)
	txID := "test-" + newTestUUID(t)[:12]

	expect(t, f.post(t, f.server.purchaseHandler, "/api/v1/purchases", "", map[string]string{"jws": "x"}),
		http.StatusUnauthorized, codeUnauthenticated)
	expect(t, f.post(t, f.server.purchaseHandler, "/api/v1/purchases", f.alice, map[string]string{"jws": ""}),
		http.StatusBadRequest, codeInvalidRequest)

	// Cadeia de outra raiz, com o nome da raiz da Apple: é o JWS forjado.
	forged := storekittest.NewChain(t, storekittest.Options{RootCommonName: "Apple Root CA - G3"})
	forgedJWS := forged.Sign(t, storekittest.Transaction(testBundle, f.productID, txID, f.alice))
	expect(t, f.post(t, f.server.purchaseHandler, "/api/v1/purchases", f.alice, map[string]string{"jws": forgedJWS}),
		http.StatusBadRequest, codePurchaseInvalid)

	rr := f.post(t, f.server.purchaseHandler, "/api/v1/purchases", f.alice, map[string]string{"jws": f.purchaseJWS(t, txID, f.alice)})
	expect(t, rr, http.StatusOK, "")
	var granted purchaseResponse
	json.Unmarshal(rr.Body.Bytes(), &granted)
	if granted.TrackID != f.trackID {
		t.Fatalf("compra liberou %q, esperava %q", granted.TrackID, f.trackID)
	}

	other := f.chain.Sign(t, storekittest.Transaction(testBundle, "com.example.other", "test-"+newTestUUID(t)[:12], f.alice))
	expect(t, f.post(t, f.server.purchaseHandler, "/api/v1/purchases", f.alice, map[string]string{"jws": other}),
		http.StatusBadRequest, codeUnknownProduct)
}

// A compra de Alice, mandada por Bob: na compra é outra conta, na restauração é a de
// uma conta ativa.
func TestAnotherAccountCannotTakeAPurchase(t *testing.T) {
	f := newStoreFixture(t)
	txID := "test-" + newTestUUID(t)[:12]
	jws := f.purchaseJWS(t, txID, f.alice)

	expect(t, f.post(t, f.server.purchaseHandler, "/api/v1/purchases", f.alice, map[string]string{"jws": jws}), http.StatusOK, "")
	expect(t, f.post(t, f.server.purchaseHandler, "/api/v1/purchases", f.bob, map[string]string{"jws": jws}),
		http.StatusForbidden, codePurchaseAccountMismatch)
	expect(t, f.post(t, f.server.restorePurchaseHandler, "/api/v1/purchases/restore", f.bob, map[string]string{"jws": jws}),
		http.StatusConflict, codePurchaseOwnedByOtherAccount)
	expect(t, f.license(t, f.bob, newTestUUID(t)), http.StatusForbidden, codeEntitlementRequired)
}

func TestARefundNotificationRevokesForGood(t *testing.T) {
	f := newStoreFixture(t)
	txID := "test-" + newTestUUID(t)[:12]
	jws := f.purchaseJWS(t, txID, f.alice)
	route := f.server.appStoreNotificationRoute(newRateLimiter(100, time.Minute))

	expect(t, f.post(t, f.server.purchaseHandler, "/api/v1/purchases", f.alice, map[string]string{"jws": jws}), http.StatusOK, "")
	expect(t, f.license(t, f.alice, newTestUUID(t)), http.StatusOK, "")

	// Notificação forjada: não revoga nada.
	forged := storekittest.NewChain(t, storekittest.Options{})
	forgedInner := forged.Sign(t, storekittest.Transaction(testBundle, f.productID, txID, f.alice))
	forgedNote := forged.Sign(t, storekittest.Notification(testBundle, "REFUND", forgedInner))
	expect(t, f.post(t, route, "/api/v1/appstore/notifications", "", map[string]string{"signedPayload": forgedNote}),
		http.StatusBadRequest, codePurchaseInvalid)
	expect(t, f.license(t, f.alice, newTestUUID(t)), http.StatusOK, "")

	refunded := storekittest.Transaction(testBundle, f.productID, txID, f.alice)
	refunded["revocationDate"] = time.Now().UnixMilli()
	note := f.chain.Sign(t, storekittest.Notification(testBundle, "REFUND", f.chain.Sign(t, refunded)))
	expect(t, f.post(t, route, "/api/v1/appstore/notifications", "", map[string]string{"signedPayload": note}), http.StatusOK, "")

	expect(t, f.license(t, f.alice, newTestUUID(t)), http.StatusForbidden, codeEntitlementRequired)
	// O JWS de antes do reembolso continua bem assinado, e não volta a valer.
	expect(t, f.post(t, f.server.purchaseHandler, "/api/v1/purchases", f.alice, map[string]string{"jws": jws}),
		http.StatusForbidden, codePurchaseRevoked)
	expect(t, f.post(t, f.server.restorePurchaseHandler, "/api/v1/purchases/restore", f.bob, map[string]string{"jws": jws}),
		http.StatusForbidden, codePurchaseRevoked)
}

func TestTheNotificationRouteHasABodyCeiling(t *testing.T) {
	f := newStoreFixture(t)
	route := f.server.appStoreNotificationRoute(newRateLimiter(100, time.Minute))

	// Abaixo do teto, um corpo grande chega à verificação e cai por assinatura; acima,
	// não chega a ser lido.
	under := `{"signedPayload":"` + strings.Repeat("a", appStoreNotificationBodyLimit/2) + `"}`
	over := `{"signedPayload":"` + strings.Repeat("a", appStoreNotificationBodyLimit+1) + `"}`
	for _, tc := range []struct {
		body string
		code string
	}{{under, codePurchaseInvalid}, {over, codeInvalidRequest}} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/appstore/notifications", strings.NewReader(tc.body))
		rr := httptest.NewRecorder()
		route(rr, req)
		expect(t, rr, http.StatusBadRequest, tc.code)
	}
}

func TestTheLicenseNeedsADeviceAndHasNoDeviceLimit(t *testing.T) {
	f := newStoreFixture(t)
	jws := f.purchaseJWS(t, "test-"+newTestUUID(t)[:12], f.alice)
	expect(t, f.post(t, f.server.purchaseHandler, "/api/v1/purchases", f.alice, map[string]string{"jws": jws}), http.StatusOK, "")

	expect(t, f.license(t, f.alice, ""), http.StatusBadRequest, codeDeviceIDRequired)
	expect(t, f.license(t, f.alice, "unknown-device"), http.StatusBadRequest, codeDeviceIDRequired)
	expect(t, f.license(t, f.alice, strings.Repeat("a", 65)), http.StatusBadRequest, codeDeviceIDRequired)

	for i := 0; i < 6; i++ {
		rr := f.license(t, f.alice, newTestUUID(t))
		expect(t, rr, http.StatusOK, "")
		var l domain.License
		json.Unmarshal(rr.Body.Bytes(), &l)
		if l.TrackID != f.trackID || len(l.KeyHex) != 64 {
			t.Fatalf("licença: %s", rr.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tracks/x/license", nil)
	req.SetPathValue("id", "x' OR '1'='1")
	req.Header.Set("Authorization", bearer(t, f.alice))
	req.Header.Set("X-Device-ID", newTestUUID(t))
	rr := httptest.NewRecorder()
	f.server.trackLicenseHandler(rr, req)
	expect(t, rr, http.StatusBadRequest, codeInvalidRequest)
}

func TestThePackageIsOnlyForBuyers(t *testing.T) {
	f := newStoreFixture(t)
	get := func(userID string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tracks/"+f.trackID+"/package", nil)
		req.SetPathValue("id", f.trackID)
		req.Header.Set("Authorization", bearer(t, userID))
		rr := httptest.NewRecorder()
		f.server.trackPackageHandler(rr, req)
		return rr
	}
	expect(t, get(f.bob), http.StatusForbidden, codeEntitlementRequired)

	jws := f.purchaseJWS(t, "test-"+newTestUUID(t)[:12], f.alice)
	expect(t, f.post(t, f.server.purchaseHandler, "/api/v1/purchases", f.alice, map[string]string{"jws": jws}), http.StatusOK, "")
	rr := get(f.alice)
	expect(t, rr, http.StatusOK, "")

	var l domain.License
	json.Unmarshal(f.license(t, f.alice, newTestUUID(t)).Body.Bytes(), &l)
	if _, err := domain.OpenTrackPackage(l.KeyHex, f.trackID, l.ContentVersion, rr.Body.Bytes()); err != nil {
		t.Fatalf("o pacote não abre com a licença: %v", err)
	}
}

func TestTheCatalogMarksWhatWasBought(t *testing.T) {
	f := newStoreFixture(t)
	owned := func(userID string) (bool, bool) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tracks", nil)
		if userID != "" {
			req.Header.Set("Authorization", bearer(t, userID))
		}
		rr := httptest.NewRecorder()
		f.server.tracksHandler(rr, req)
		var tracks []domain.Track
		json.Unmarshal(rr.Body.Bytes(), &tracks)
		for _, tr := range tracks {
			if tr.ID == f.trackID {
				return true, tr.Owned
			}
		}
		return false, false
	}

	jws := f.purchaseJWS(t, "test-"+newTestUUID(t)[:12], f.alice)
	expect(t, f.post(t, f.server.purchaseHandler, "/api/v1/purchases", f.alice, map[string]string{"jws": jws}), http.StatusOK, "")
	if listed, own := owned(""); !listed || own {
		t.Fatalf("sem conta: listada=%v comprada=%v", listed, own)
	}
	if _, own := owned(f.alice); !own {
		t.Fatal("quem comprou não vê a compra")
	}

	// Descontinuada: some para quem não comprou, fica para quem comprou.
	f.conn.Exec(context.Background(), `UPDATE tracks SET status = 'discontinued' WHERE id = $1`, f.trackID)
	if listed, _ := owned(f.bob); listed {
		t.Fatal("descontinuada aparece para quem não comprou")
	}
	if listed, own := owned(f.alice); !listed || !own {
		t.Fatal("descontinuada sumiu de quem comprou")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tracks", nil)
	req.Header.Set("Authorization", "Bearer invalido")
	rr := httptest.NewRecorder()
	f.server.tracksHandler(rr, req)
	expect(t, rr, http.StatusUnauthorized, codeUnauthenticated)
}

// Trilha indisponível pela rota (ADR 0014): some do catálogo de quem não testa nem
// comprou. Com conta, as rotas de conteúdo respondem só para ela, e token inválido é
// 401, não a resposta do visitante.
func TestTheContentRoutesHideAnUnavailableTrack(t *testing.T) {
	f := newStoreFixture(t)
	ctx := context.Background()
	f.conn.Exec(ctx, `UPDATE tracks SET available = false WHERE id = $1`, f.trackID)
	if _, err := f.conn.Exec(ctx, `INSERT INTO track_previewers (track_id, user_id) VALUES ($1, $2)`, f.trackID, f.alice); err != nil {
		t.Fatal(err)
	}

	get := func(handler http.HandlerFunc, path, authorization string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Accept-Language", "pt-BR")
		if authorization != "" {
			req.Header.Set("Authorization", authorization)
		}
		rr := httptest.NewRecorder()
		handler(rr, req)
		return rr
	}
	listed := func(authorization string) bool {
		rr := get(f.server.tracksHandler, "/api/v1/tracks", authorization)
		var tracks []domain.Track
		json.Unmarshal(rr.Body.Bytes(), &tracks)
		for _, tr := range tracks {
			if tr.ID == f.trackID {
				return true
			}
		}
		return false
	}
	if listed("") || listed(bearer(t, f.bob)) {
		t.Fatal("indisponível aparece para quem não testa")
	}
	if !listed(bearer(t, f.alice)) {
		t.Fatal("indisponível some de quem testa")
	}

	for path, handler := range map[string]http.HandlerFunc{
		"/api/v1/nodes":      f.server.getNodesHandler,
		"/api/v1/challenges": f.server.challengesHandler,
	} {
		expect(t, get(handler, path, "Bearer invalido"), http.StatusUnauthorized, codeUnauthenticated)
		if cc := get(handler, path, bearer(t, f.alice)).Header().Get("Cache-Control"); cc != "private, no-store" {
			t.Errorf("%s com conta saiu com Cache-Control %q", path, cc)
		}
		if cc := get(handler, path, "").Header().Get("Cache-Control"); cc != "no-cache" {
			t.Errorf("%s sem conta saiu com Cache-Control %q", path, cc)
		}
	}
}

// O catálogo abre pela principal, e a trilha revogada fica com o motivo, para a tela
// oferecer a recompra (LogN Trilhas, estado "revogada").
func TestTheCatalogLeadsWithTheFreeTrackAndShowsRevocation(t *testing.T) {
	f := newStoreFixture(t)
	catalog := func(userID string) []domain.Track {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tracks", nil)
		req.Header.Set("Accept-Language", "pt-BR")
		req.Header.Set("Authorization", bearer(t, userID))
		rr := httptest.NewRecorder()
		f.server.tracksHandler(rr, req)
		var tracks []domain.Track
		json.Unmarshal(rr.Body.Bytes(), &tracks)
		return tracks
	}

	tracks := catalog(f.alice)
	if len(tracks) == 0 || tracks[0].Kind != "free" || tracks[0].ProductID != "" || tracks[0].Color == "" {
		t.Fatalf("a principal não abre o catálogo: %+v", tracks)
	}

	txID := "test-" + newTestUUID(t)[:12]
	expect(t, f.post(t, f.server.purchaseHandler, "/api/v1/purchases", f.alice, map[string]string{"jws": f.purchaseJWS(t, txID, f.alice)}), http.StatusOK, "")
	if err := f.server.repo.RevokeTransaction(context.Background(), domain.ProviderAppleStoreKit, txID, "refund", 1000); err != nil {
		t.Fatal(err)
	}
	for _, tr := range catalog(f.alice) {
		if tr.ID == f.trackID {
			if tr.Owned || tr.RevokedReason != "refund" || len(tr.Languages) != 1 || tr.Languages[0] != "pt-BR" {
				t.Fatalf("trilha revogada no catálogo: %+v", tr)
			}
			return
		}
	}
	t.Fatal("a trilha revogada sumiu do catálogo")
}
