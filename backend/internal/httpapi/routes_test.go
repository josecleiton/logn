package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Os testes de handler montam o `Server` direto. Estes passam por `New`, que é o que o
// `main` sobe: provam que limite e origem estão ligados às rotas, não só que existem.

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	h, err := New(Deps{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h
}

func serve(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

// Uma senha acima do teto de corpo não chega inteira ao handler: o JSON cortado vira
// 400. Sem o `limitBody`, ela passaria no decode e daria 401 pela senha longa.
func TestNewLimitsTheAuthBody(t *testing.T) {
	h := newTestHandler(t)
	body := `{"email":"a@example.com","password":"` + strings.Repeat("x", authBodyLimit+1) + `"}`

	rr := serve(h, http.MethodPost, "/api/v1/auth/login", body)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), codeInvalidRequest) {
		t.Fatalf("got %d %s, want 400 %s", rr.Code, rr.Body.String(), codeInvalidRequest)
	}
}

// As rotas de auth dividem um balde de 30 por minuto por IP: estourar pelo login fecha
// também o cadastro.
func TestNewSharesTheAuthRateLimit(t *testing.T) {
	h := newTestHandler(t)
	for i := 0; i < 30; i++ {
		if rr := serve(h, http.MethodPost, "/api/v1/auth/login", "não é json"); rr.Code == http.StatusTooManyRequests {
			t.Fatalf("pedido %d já barrado", i+1)
		}
	}

	rr := serve(h, http.MethodPost, "/api/v1/auth/register", "não é json")
	if rr.Code != http.StatusTooManyRequests || !strings.Contains(rr.Body.String(), codeRateLimited) {
		t.Fatalf("31º pedido: got %d %s, want 429 %s", rr.Code, rr.Body.String(), codeRateLimited)
	}
}

// Com a verificação de origem ligada, só as sondas, a rota interna e a notificação da
// App Store passam sem o segredo do proxy. As isentas respondem pelo próprio mux (GET
// numa rota de POST é 405), o que prova que passaram da checagem.
func TestNewAppliesOriginVerification(t *testing.T) {
	t.Setenv("K_SERVICE", "")
	t.Setenv("ORIGIN_TRUSTED_CIDRS", "173.245.48.0/20")
	t.Setenv("ORIGIN_SHARED_SECRET", "s3cr3t")
	h := newTestHandler(t)

	cases := []struct {
		path string
		want int
	}{
		{"/health", http.StatusOK},
		{"/api/v1/internal/purge", http.StatusMethodNotAllowed},
		{"/api/v1/appstore/notifications", http.StatusMethodNotAllowed},
		{"/api/v1/nodes", http.StatusForbidden},
		{"/api/v1/auth/login", http.StatusForbidden},
	}
	for _, tc := range cases {
		if rr := serve(h, http.MethodGet, tc.path, ""); rr.Code != tc.want {
			t.Errorf("GET %s: got %d, want %d", tc.path, rr.Code, tc.want)
		}
	}
}

// No Cloud Run a verificação de origem é obrigatória, e fora dele configuração pela
// metade não sobe: `New` devolve erro para o `main` abortar.
func TestNewRefusesMissingOriginConfig(t *testing.T) {
	cases := []struct {
		name, kService, cidrs, secret string
	}{
		{"Cloud Run sem nada", "logn", "", ""},
		{"Cloud Run sem segredo", "logn", "173.245.48.0/20", ""},
		{"local só com segredo", "", "", "s3cr3t"},
		{"local só com CIDR", "", "173.245.48.0/20", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("K_SERVICE", tc.kService)
			t.Setenv("ORIGIN_TRUSTED_CIDRS", tc.cidrs)
			t.Setenv("ORIGIN_SHARED_SECRET", tc.secret)
			if _, err := New(Deps{}); err == nil {
				t.Fatal("New subiu sem verificação de origem válida")
			}
		})
	}
}
