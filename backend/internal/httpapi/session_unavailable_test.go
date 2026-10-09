package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/josecleiton/logn/backend/internal/domain"
)

// Banco fora não é sessão inválida. Com 401, o app apagava a sessão e deslogava o
// jogador por causa de um pool cheio; tem de sair 503, e o app tenta de novo.
func TestDatabaseDownIsNotAnInvalidSession(t *testing.T) {
	pool := setupTestDB(t)
	pool.Close()
	s := &Server{repo: domain.NewRepository(pool)}

	domain.JwtSecretKey = []byte("test-secret")
	token, err := domain.GenerateAccessToken("00000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatal(err)
	}

	check := func(name string, rec *httptest.ResponseRecorder) {
		t.Helper()
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s: status %d, esperava 503", name, rec.Code)
		}
		var body apiError
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil || body.Code != codeInternal {
			t.Fatalf("%s: corpo %+v (%v)", name, body, err)
		}
		if rec.Header().Get("Retry-After") == "" {
			t.Errorf("%s: sem Retry-After", name)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/progress", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	if _, ok := s.authenticate(rec, req); ok {
		t.Fatal("autenticou sem banco")
	}
	check("rota autenticada", rec)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh",
		strings.NewReader(`{"refresh_token":"qualquer"}`))
	rec = httptest.NewRecorder()
	s.refreshHandler(rec, req)
	check("refresh", rec)
}
