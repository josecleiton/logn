package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/josecleiton/logn/backend/internal/googleplay"
)

// fakePlay é a Google Play Developer API com as compras que o teste montou.
type fakePlay struct {
	mu        sync.Mutex
	purchases map[string]googleplay.ProductPurchase // por token
	err       error
	ackErr    error
	// Com ackErr, o reconhecimento chega à loja mesmo assim (a resposta se perdeu).
	ackLands bool
	acked    []string
	voided   []googleplay.VoidedPurchase
}

func (p *fakePlay) Product(_ context.Context, productID, token string) (googleplay.ProductPurchase, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil {
		return googleplay.ProductPurchase{}, p.err
	}
	got, ok := p.purchases[productID+"/"+token]
	if !ok {
		return googleplay.ProductPurchase{}, googleplay.ErrNotFound
	}
	got.Raw = []byte(`{"kind":"androidpublisher#productPurchase"}`)
	return got, nil
}

func (p *fakePlay) Acknowledge(_ context.Context, productID, token string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	key := productID + "/" + token
	if p.ackErr != nil {
		if p.ackLands {
			got := p.purchases[key]
			got.AcknowledgementState = 1
			p.purchases[key] = got
		}
		return p.ackErr
	}
	p.acked = append(p.acked, token)
	got := p.purchases[key]
	got.AcknowledgementState = 1
	p.purchases[key] = got
	return nil
}

func (p *fakePlay) Voided(context.Context, time.Time) ([]googleplay.VoidedPurchase, error) {
	return p.voided, p.err
}

func newPlayToken(t *testing.T) string { return "token-" + newTestUUID(t) }

func playFixture(t *testing.T) (*storeFixture, *fakePlay) {
	f := newStoreFixture(t)
	play := &fakePlay{purchases: map[string]googleplay.ProductPurchase{}}
	f.server.play = play
	return f, play
}

func playBody(product, token string) map[string]string {
	return map[string]string{"provider": "google_play", "product_id": product, "purchase_token": token}
}

func TestPlayPurchase(t *testing.T) {
	f, play := playFixture(t)
	token := newPlayToken(t)
	play.purchases[f.productID+"/"+token] = googleplay.ProductPurchase{ObfuscatedExternalAccountID: f.alice}
	t.Cleanup(func() {
		f.conn.Exec(context.Background(), `DELETE FROM revoked_transactions WHERE original_transaction_id = $1`, googleplay.TransactionKey(token))
	})

	rr := f.post(t, f.server.purchaseHandler, "/api/v1/purchases", f.alice, playBody(f.productID, token))
	expect(t, rr, http.StatusOK, "")
	if len(play.acked) != 1 || play.acked[0] != token {
		t.Fatalf("a compra não foi reconhecida: %v", play.acked)
	}

	var provider, otid, env string
	if err := f.conn.QueryRow(context.Background(),
		`SELECT e.provider, e.original_transaction_id, st.environment FROM entitlements e
		 JOIN store_transactions st ON st.provider = e.provider AND st.original_transaction_id = e.original_transaction_id
		 WHERE e.user_id = $1 AND e.track_id = $2 AND e.status = 'active'`, f.alice, f.trackID).Scan(&provider, &otid, &env); err != nil {
		t.Fatalf("licença não gravada: %v", err)
	}
	if provider != "google_play" || otid != googleplay.TransactionKey(token) || env != googleplay.EnvironmentProduction {
		t.Errorf("licença gravada como %s/%s/%s", provider, otid, env)
	}

	// Mandar de novo não duplica nada, e não reconhece de novo o que já foi reconhecido.
	expect(t, f.post(t, f.server.purchaseHandler, "/api/v1/purchases", f.alice, playBody(f.productID, token)), http.StatusOK, "")
	if len(play.acked) != 1 {
		t.Errorf("reconheceu duas vezes: %v", play.acked)
	}

	// A compra é de Alice: Bob não a leva, nem comprando nem restaurando.
	expect(t, f.post(t, f.server.purchaseHandler, "/api/v1/purchases", f.bob, playBody(f.productID, token)),
		http.StatusForbidden, codePurchaseAccountMismatch)
	expect(t, f.post(t, f.server.restorePurchaseHandler, "/api/v1/purchases/restore", f.bob, playBody(f.productID, token)),
		http.StatusConflict, codePurchaseOwnedByOtherAccount)
}

func TestPlayPurchaseRefuses(t *testing.T) {
	f, play := playFixture(t)
	pending, cancelled, consumed, rewarded, test := newPlayToken(t), newPlayToken(t), newPlayToken(t), newPlayToken(t), newPlayToken(t)
	reward := 2
	testType := 0
	play.purchases[f.productID+"/"+pending] = googleplay.ProductPurchase{PurchaseState: 2, ObfuscatedExternalAccountID: f.alice}
	play.purchases[f.productID+"/"+cancelled] = googleplay.ProductPurchase{PurchaseState: 1, ObfuscatedExternalAccountID: f.alice}
	play.purchases[f.productID+"/"+consumed] = googleplay.ProductPurchase{ConsumptionState: 1, ObfuscatedExternalAccountID: f.alice}
	play.purchases[f.productID+"/"+rewarded] = googleplay.ProductPurchase{PurchaseType: &reward, ObfuscatedExternalAccountID: f.alice}
	play.purchases[f.productID+"/"+test] = googleplay.ProductPurchase{PurchaseType: &testType, ObfuscatedExternalAccountID: f.bob}

	post := func(user string, body map[string]string) *httptest.ResponseRecorder {
		return f.post(t, f.server.purchaseHandler, "/api/v1/purchases", user, body)
	}
	expect(t, post("", playBody(f.productID, pending)), http.StatusUnauthorized, codeUnauthenticated)
	expect(t, post(f.alice, playBody(f.productID, pending)), http.StatusConflict, codePurchasePending)
	expect(t, post(f.alice, playBody(f.productID, cancelled)), http.StatusBadRequest, codePurchaseInvalid)
	expect(t, post(f.alice, playBody(f.productID, consumed)), http.StatusBadRequest, codePurchaseInvalid)
	expect(t, post(f.alice, playBody(f.productID, rewarded)), http.StatusBadRequest, codePurchaseInvalid)
	// Token que a loja não conhece: inventado ou de outro app.
	expect(t, post(f.alice, playBody(f.productID, newPlayToken(t))), http.StatusBadRequest, codePurchaseInvalid)
	// Produto fora do catálogo não chega a ir à loja.
	expect(t, post(f.alice, playBody("com.example.other", pending)), http.StatusBadRequest, codeUnknownProduct)
	// Entrada fora do formato.
	for _, body := range []map[string]string{
		playBody(f.productID, ""),
		playBody(f.productID, "../../../x-long-enough"),
		playBody("", pending),
		playBody("../../x", pending),
		{"provider": "stripe", "product_id": f.productID, "purchase_token": pending},
	} {
		expect(t, post(f.alice, body), http.StatusBadRequest, codeInvalidRequest)
	}
	if len(play.acked) != 0 {
		t.Errorf("compra recusada foi reconhecida: %v", play.acked)
	}

	// Compra de testador de licença libera, em `Test`.
	expect(t, post(f.bob, playBody(f.productID, test)), http.StatusOK, "")
	var env string
	f.conn.QueryRow(context.Background(), `SELECT environment FROM store_transactions WHERE original_transaction_id = $1`,
		googleplay.TransactionKey(test)).Scan(&env)
	if env != googleplay.EnvironmentTest {
		t.Errorf("ambiente %q", env)
	}

	// Erro da loja que não é "não existe" é falha nossa: 502, e o app tenta de novo.
	play.err = errors.New("googleplay: a API respondeu 503")
	expect(t, post(f.alice, playBody(f.productID, pending)), http.StatusBadGateway, codeInternal)
	play.err = nil

	// Play desligado.
	f.server.play = nil
	expect(t, post(f.alice, playBody(f.productID, pending)), http.StatusServiceUnavailable, codeStoreUnavailable)
}

// Token de um item barato do mesmo app não abre trilha: a loja diz de que produto é a
// compra, e ela manda.
func TestPlayPurchaseOfAnotherProduct(t *testing.T) {
	f, play := playFixture(t)
	tip, bulk := newPlayToken(t), newPlayToken(t)
	play.purchases[f.productID+"/"+tip] = googleplay.ProductPurchase{ProductID: "com.example.logn.tip", ObfuscatedExternalAccountID: f.alice}
	play.purchases[f.productID+"/"+bulk] = googleplay.ProductPurchase{ProductID: f.productID, Quantity: 5, ObfuscatedExternalAccountID: f.alice}
	for _, tok := range []string{tip, bulk} {
		expect(t, f.post(t, f.server.purchaseHandler, "/api/v1/purchases", f.alice, playBody(f.productID, tok)),
			http.StatusBadRequest, codePurchaseInvalid)
	}
	if len(play.acked) != 0 {
		t.Errorf("reconheceu compra de outro produto: %v", play.acked)
	}
}

// Compra sem a conta dentro (código promocional resgatado na loja) não entra pela compra,
// só pela restauração; e restauração de conta que já não existe vale.
func TestPlayAccountRules(t *testing.T) {
	f, play := playFixture(t)
	promo, orphan := newPlayToken(t), newPlayToken(t)
	gone := newTestUUID(t)
	play.purchases[f.productID+"/"+promo] = googleplay.ProductPurchase{}
	play.purchases[f.productID+"/"+orphan] = googleplay.ProductPurchase{ObfuscatedExternalAccountID: gone}

	expect(t, f.post(t, f.server.purchaseHandler, "/api/v1/purchases", f.alice, playBody(f.productID, promo)),
		http.StatusForbidden, codePurchaseAccountMismatch)
	expect(t, f.post(t, f.server.purchaseHandler, "/api/v1/purchases", f.bob, playBody(f.productID, orphan)),
		http.StatusForbidden, codePurchaseAccountMismatch)
	expect(t, f.post(t, f.server.restorePurchaseHandler, "/api/v1/purchases/restore", f.bob, playBody(f.productID, orphan)),
		http.StatusOK, "")
	// A trilha já é de Bob pela outra compra; a de código promocional restaura em Alice.
	expect(t, f.post(t, f.server.restorePurchaseHandler, "/api/v1/purchases/restore", f.alice, playBody(f.productID, promo)),
		http.StatusOK, "")
	// E, com dona ativa, não passa para Bob.
	expect(t, f.post(t, f.server.restorePurchaseHandler, "/api/v1/purchases/restore", f.bob, playBody(f.productID, promo)),
		http.StatusConflict, codePurchaseOwnedByOtherAccount)
}

// O reconhecimento que falhou mas chegou à loja (resposta perdida, dois pedidos ao mesmo
// tempo) vale como feito.
func TestPlayAcknowledgeThatLanded(t *testing.T) {
	f, play := playFixture(t)
	token := newPlayToken(t)
	play.purchases[f.productID+"/"+token] = googleplay.ProductPurchase{ObfuscatedExternalAccountID: f.alice}
	play.ackErr = &googleplay.APIError{Status: http.StatusBadRequest, Reason: "?"}
	play.ackLands = true
	expect(t, f.post(t, f.server.purchaseHandler, "/api/v1/purchases", f.alice, playBody(f.productID, token)),
		http.StatusOK, "")
}

func TestPlayAcknowledgeFailureIsRetried(t *testing.T) {
	f, play := playFixture(t)
	token := newPlayToken(t)
	play.purchases[f.productID+"/"+token] = googleplay.ProductPurchase{ObfuscatedExternalAccountID: f.alice}
	play.ackErr = errors.New("googleplay: a API respondeu 500")

	expect(t, f.post(t, f.server.purchaseHandler, "/api/v1/purchases", f.alice, playBody(f.productID, token)),
		http.StatusBadGateway, codeInternal)
	// A licença ficou; o app manda de novo e o reconhecimento sai.
	play.ackErr = nil
	expect(t, f.post(t, f.server.purchaseHandler, "/api/v1/purchases", f.alice, playBody(f.productID, token)),
		http.StatusOK, "")
	if len(play.acked) != 1 {
		t.Errorf("reconhecimentos: %v", play.acked)
	}
}

func TestPlayVoided(t *testing.T) {
	f, play := playFixture(t)
	f.server.cloudValidator = &mockCloudValidator{}
	refunded, charged := newPlayToken(t), newPlayToken(t)
	unknown := newPlayToken(t)
	for _, tok := range []string{refunded, charged} {
		play.purchases[f.productID+"/"+tok] = googleplay.ProductPurchase{ObfuscatedExternalAccountID: f.alice}
	}
	t.Cleanup(func() {
		for _, tok := range []string{refunded, charged, unknown} {
			f.conn.Exec(context.Background(), `DELETE FROM revoked_transactions WHERE original_transaction_id = $1`, googleplay.TransactionKey(tok))
		}
	})
	expect(t, f.post(t, f.server.purchaseHandler, "/api/v1/purchases", f.alice, playBody(f.productID, refunded)), http.StatusOK, "")

	play.voided = []googleplay.VoidedPurchase{
		{PurchaseToken: refunded, VoidedTimeMillis: "1000", VoidedReason: 1},
		{PurchaseToken: charged, VoidedTimeMillis: "2000", VoidedReason: 7},
		{PurchaseToken: unknown, VoidedTimeMillis: "3000"},
		{PurchaseToken: ""},
	}

	call := func(auth string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/internal/purge", nil)
		if auth != "" {
			req.Header.Set("Authorization", "Bearer "+auth)
		}
		f.server.purgeHandler(rr, req)
		return rr
	}

	internalEnv(t, testAudience, testSchedulerAccount, testAdminAccount)
	// Só a conta do Scheduler chama.
	if rr := call(""); rr.Code != http.StatusUnauthorized {
		t.Fatalf("sem token: %d", rr.Code)
	}
	for _, other := range []string{testAdminAccount, "attacker@other-project.iam.gserviceaccount.com"} {
		if rr := call(other); rr.Code != http.StatusForbidden {
			t.Fatalf("%s: %d", other, rr.Code)
		}
	}
	internalEnv(t, "", testSchedulerAccount, testAdminAccount)
	if rr := call(testSchedulerAccount); rr.Code != http.StatusForbidden {
		t.Fatalf("sem audiência configurada: %d", rr.Code)
	}
	internalEnv(t, testAudience, testSchedulerAccount, testAdminAccount)

	if rr := call(testSchedulerAccount); rr.Code != http.StatusOK {
		t.Fatalf("Scheduler: %d %s", rr.Code, rr.Body.String())
	}
	var status, reason string
	f.conn.QueryRow(context.Background(), `SELECT status, revoked_reason FROM entitlements WHERE user_id = $1 AND track_id = $2`,
		f.alice, f.trackID).Scan(&status, &reason)
	if status != "revoked" || reason != "refund" {
		t.Errorf("licença depois do reembolso: %s/%s", status, reason)
	}
	for tok, want := range map[string]string{charged: "fraud", unknown: "refund"} {
		var got string
		f.conn.QueryRow(context.Background(), `SELECT reason FROM revoked_transactions WHERE provider = 'google_play' AND original_transaction_id = $1`,
			googleplay.TransactionKey(tok)).Scan(&got)
		if got != want {
			t.Errorf("anulada gravada como %q, esperava %q", got, want)
		}
	}
	// A compra estornada não volta por compra nem por restauração.
	expect(t, f.post(t, f.server.purchaseHandler, "/api/v1/purchases", f.alice, playBody(f.productID, charged)),
		http.StatusForbidden, codePurchaseRevoked)
	expect(t, f.post(t, f.server.restorePurchaseHandler, "/api/v1/purchases/restore", f.alice, playBody(f.productID, refunded)),
		http.StatusForbidden, codePurchaseRevoked)

	// Rodar de novo não muda nada.
	if rr := call(testSchedulerAccount); rr.Code != http.StatusOK {
		t.Fatalf("segunda rodada: %d", rr.Code)
	}

	// Compra gravada cujo reconhecimento falhou e o app não mandou de novo: o job
	// reconhece, antes de o Play estornar sozinho.
	late := newPlayToken(t)
	play.purchases[f.productID+"/"+late] = googleplay.ProductPurchase{ObfuscatedExternalAccountID: f.bob}
	play.ackErr = errors.New("googleplay: pedido falhou")
	expect(t, f.post(t, f.server.purchaseHandler, "/api/v1/purchases", f.bob, playBody(f.productID, late)),
		http.StatusBadGateway, codeInternal)
	play.ackErr = nil
	before := len(play.acked)
	if rr := call(testSchedulerAccount); rr.Code != http.StatusOK {
		t.Fatalf("rodada com pendente: %d", rr.Code)
	}
	if len(play.acked) != before+1 || play.acked[len(play.acked)-1] != late {
		t.Errorf("o job não reconheceu a compra pendente: %v", play.acked)
	}
	// Loja fora: 500, e o Scheduler tenta de novo.
	play.err = errors.New("googleplay: a API respondeu 503")
	if rr := call(testSchedulerAccount); rr.Code != http.StatusInternalServerError {
		t.Fatalf("loja fora: %d", rr.Code)
	}
}
