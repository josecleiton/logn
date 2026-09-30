package googleplay

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type staticToken string

func (s staticToken) Token(context.Context) (string, error) { return string(s), nil }

const (
	testPackage = "com.example.app"
	testProduct = "com.example.app.track.t"
	testToken   = "abcdefghijklmnop.AO-J1Oz_example_token"
)

func intp(v int) *int { return &v }

func TestEnvironment(t *testing.T) {
	cases := []struct {
		name string
		p    ProductPurchase
		env  string
		err  error
	}{
		{"compra de verdade", ProductPurchase{}, EnvironmentProduction, nil},
		{"código promocional", ProductPurchase{PurchaseType: intp(1)}, EnvironmentProduction, nil},
		{"testador de licença", ProductPurchase{PurchaseType: intp(0)}, EnvironmentTest, nil},
		{"recompensa", ProductPurchase{PurchaseType: intp(2)}, "", ErrNotPurchased},
		{"pendente", ProductPurchase{PurchaseState: 2}, "", ErrPending},
		{"cancelada", ProductPurchase{PurchaseState: 1}, "", ErrNotPurchased},
		{"consumida", ProductPurchase{ConsumptionState: 1}, "", ErrNotPurchased},
		{"estado desconhecido", ProductPurchase{PurchaseState: 9}, "", ErrNotPurchased},
	}
	for _, c := range cases {
		env, err := c.p.Environment()
		if env != c.env || !errors.Is(err, c.err) {
			t.Errorf("%s: %q, %v; esperava %q, %v", c.name, env, err, c.env, c.err)
		}
	}
}

func TestValidInput(t *testing.T) {
	if !ValidInput(testProduct, testToken) {
		t.Fatal("produto e token válidos recusados")
	}
	for _, c := range [][2]string{
		{"", testToken},
		{"Com.Example", testToken},
		{"../other", testToken},
		{"a/b", testToken},
		{testProduct, ""},
		{testProduct, "short"},
		{testProduct, "abcdefghijklmnop/../../x"},
		{testProduct, "abcdefghijklmnop?x=1"},
		{testProduct, "abcdefghijklmnop%2F"},
		{testProduct, strings.Repeat("a", 4097)},
	} {
		if ValidInput(c[0], c[1]) {
			t.Errorf("aceitou produto=%q token=%q", c[0], c[1])
		}
	}
}

func TestTransactionKey(t *testing.T) {
	k := TransactionKey(testToken)
	if len(k) != 64 || k != TransactionKey(testToken) || k == TransactionKey(testToken+"x") {
		t.Fatalf("chave %q", k)
	}
}

func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := NewClient(testPackage, srv.URL, staticToken("access"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestProduct(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		want := "/androidpublisher/v3/applications/" + testPackage + "/purchases/products/" + testProduct + "/tokens/" + testToken
		if r.Method != http.MethodGet || r.URL.Path != want {
			t.Errorf("%s %s; esperava GET %s", r.Method, r.URL.Path, want)
		}
		if r.Header.Get("Authorization") != "Bearer access" {
			t.Errorf("sem o token de acesso")
		}
		w.Write([]byte(`{"kind":"androidpublisher#productPurchase","purchaseTimeMillis":"1","purchaseState":0,
			"consumptionState":0,"acknowledgementState":0,"orderId":"GPA.1","obfuscatedExternalAccountId":"acc"}`))
	})
	p, err := c.Product(context.Background(), testProduct, testToken)
	if err != nil {
		t.Fatal(err)
	}
	if p.ObfuscatedExternalAccountID != "acc" || !p.NeedsAcknowledge() || p.PurchaseType != nil || len(p.Raw) == 0 {
		t.Errorf("compra lida errado: %+v", p)
	}
}

func apiErrorBody(status int, reason string) string {
	return fmt.Sprintf(`{"error":{"code":%d,"message":"x","errors":[{"message":"x","domain":"androidpublisher","reason":%q}]}}`, status, reason)
}

func TestProductErrors(t *testing.T) {
	// O token que não vale é compra inválida.
	for _, c := range []struct {
		status int
		reason string
	}{
		{http.StatusGone, ""},
		{http.StatusBadRequest, "invalid"},
		{http.StatusBadRequest, "purchaseTokenDoesNotMatchProductId"},
		{http.StatusNotFound, "purchaseTokenNotFound"},
	} {
		cl := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(c.status)
			w.Write([]byte(apiErrorBody(c.status, c.reason)))
		})
		if _, err := cl.Product(context.Background(), testProduct, testToken); !errors.Is(err, ErrNotFound) {
			t.Errorf("%d %s: %v", c.status, c.reason, err)
		}
	}
	// O resto é falha nossa ou da loja, e o app tenta de novo: pacote errado (404 sem
	// motivo de token), conta sem permissão, API desligada, loja fora.
	for _, c := range []struct {
		status int
		reason string
	}{
		{http.StatusNotFound, "applicationNotFound"},
		{http.StatusNotFound, ""},
		{http.StatusBadRequest, ""},
		{http.StatusUnauthorized, "authError"},
		{http.StatusForbidden, "permissionDenied"},
		{http.StatusForbidden, "accessNotConfigured"},
		{http.StatusInternalServerError, ""},
	} {
		cl := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(c.status)
			w.Write([]byte(apiErrorBody(c.status, c.reason)))
		})
		_, err := cl.Product(context.Background(), testProduct, testToken)
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.Status != c.status || errors.Is(err, ErrNotFound) {
			t.Errorf("%d %s: %v", c.status, c.reason, err)
		}
	}
	// Entrada fora do formato não chega à loja.
	var called atomic.Bool
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { called.Store(true) })
	if _, err := c.Product(context.Background(), testProduct, "../../x"); !errors.Is(err, ErrBadInput) || called.Load() {
		t.Errorf("entrada ruim: %v, chamou=%v", err, called.Load())
	}
}

// O erro vai para o log, e o token de compra não pode ir junto.
func TestErrorsNeverCarryTheToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // conexão recusada: o *url.Error teria a URL inteira
	c, _ := NewClient(testPackage, srv.URL, staticToken("x"))
	_, err := c.Product(context.Background(), testProduct, testToken)
	if err == nil || strings.Contains(err.Error(), testToken) {
		t.Fatalf("erro com o token: %v", err)
	}
	if err := c.Acknowledge(context.Background(), testProduct, testToken); err == nil || strings.Contains(err.Error(), testToken) {
		t.Fatalf("erro do reconhecimento com o token: %v", err)
	}
	cl := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"code":400,"message":"token ` + testToken + `","errors":[{"reason":"` + testToken + `"}]}}`))
	})
	if _, err := cl.Product(context.Background(), testProduct, testToken); err == nil || strings.Contains(err.Error(), testToken) {
		t.Fatalf("erro da API com o token: %v", err)
	}
}

func TestForProduct(t *testing.T) {
	for _, c := range []struct {
		p    ProductPurchase
		want bool
	}{
		{ProductPurchase{}, true},
		{ProductPurchase{ProductID: testProduct, Quantity: 1}, true},
		{ProductPurchase{ProductID: "com.example.app.tip"}, false},
		{ProductPurchase{ProductID: testProduct, Quantity: 3}, false},
	} {
		if got := c.p.ForProduct(testProduct); got != c.want {
			t.Errorf("%+v: %v", c.p, got)
		}
	}
}

func TestUnauthorizedDropsTheCachedToken(t *testing.T) {
	var calls atomic.Int32
	meta := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Write([]byte(`{"access_token":"tok","expires_in":3600}`))
	}))
	t.Cleanup(meta.Close)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(api.Close)
	c, _ := NewClient(testPackage, api.URL, &MetadataTokenSource{http: meta.Client(), url: meta.URL})
	c.Product(context.Background(), testProduct, testToken)
	c.Product(context.Background(), testProduct, testToken)
	if calls.Load() != 2 {
		t.Errorf("o token recusado continuou no cache: %d buscas", calls.Load())
	}
}

func TestAcknowledge(t *testing.T) {
	var got string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.Method + " " + r.URL.Path
		w.Write([]byte(`{}`))
	})
	if err := c.Acknowledge(context.Background(), testProduct, testToken); err != nil {
		t.Fatal(err)
	}
	want := "POST /androidpublisher/v3/applications/" + testPackage + "/purchases/products/" + testProduct + "/tokens/" + testToken + ":acknowledge"
	if got != want {
		t.Errorf("%s; esperava %s", got, want)
	}
}

func TestVoidedPaginates(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("type") != "0" || r.URL.Query().Get("startTime") == "" {
			t.Errorf("consulta sem tipo ou início: %s", r.URL.RawQuery)
		}
		switch r.URL.Query().Get("token") {
		case "":
			w.Write([]byte(`{"voidedPurchases":[{"purchaseToken":"a","voidedTimeMillis":"10","voidedReason":1}],
				"tokenPagination":{"nextPageToken":"p2"}}`))
		case "p2":
			w.Write([]byte(`{"voidedPurchases":[{"purchaseToken":"b","voidedTimeMillis":"20","voidedReason":7}]}`))
		default:
			t.Errorf("página inesperada")
		}
	})
	v, err := c.Voided(context.Background(), time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 2 || v[0].Fraud() || !v[1].Fraud() || v[1].VoidedAtMs() != 20 {
		t.Errorf("anuladas: %+v", v)
	}
}

func TestVoidedStopsOnEndlessPages(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"voidedPurchases":[],"tokenPagination":{"nextPageToken":"again"}}`))
	})
	if _, err := c.Voided(context.Background(), time.Now()); err == nil {
		t.Error("paginação sem fim não parou")
	}
}

func TestMetadataTokenIsCached(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Metadata-Flavor") != "Google" {
			t.Error("sem Metadata-Flavor")
		}
		if !strings.Contains(r.URL.RawQuery, "androidpublisher") {
			t.Errorf("sem o escopo: %s", r.URL.RawQuery)
		}
		calls.Add(1)
		w.Write([]byte(`{"access_token":"tok","expires_in":3600,"token_type":"Bearer"}`))
	}))
	t.Cleanup(srv.Close)
	m := &MetadataTokenSource{http: srv.Client(), url: srv.URL + "/token?scopes=" + scope}
	for i := 0; i < 3; i++ {
		if tok, err := m.Token(context.Background()); err != nil || tok != "tok" {
			t.Fatalf("%q %v", tok, err)
		}
	}
	if calls.Load() != 1 {
		t.Errorf("%d pedidos ao servidor de metadados; esperava 1", calls.Load())
	}
}

func TestNewClientRejectsBadPackage(t *testing.T) {
	for _, p := range []string{"", "app", "Com.Example", "com.example/../x"} {
		if _, err := NewClient(p, DefaultBaseURL, staticToken("x")); err == nil {
			t.Errorf("aceitou pacote %q", p)
		}
	}
}
