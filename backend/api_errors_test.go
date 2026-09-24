package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/josecleiton/logn/backend/internal/domain"
)

func decodeAPIError(t *testing.T, rec *httptest.ResponseRecorder) apiError {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q", ct)
	}
	var e apiError
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatalf("corpo não é JSON: %q", rec.Body.String())
	}
	return e
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	rec.Header().Set("Retry-After", "60")
	writeError(rec, http.StatusTooManyRequests, codeRateLimited)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") != "60" {
		t.Error("o Retry-After definido antes se perdeu")
	}
	e := decodeAPIError(t, rec)
	if e.Code != "rate_limited" || e.Message != "Too Many Requests" {
		t.Errorf("corpo = %+v", e)
	}
}

func TestPasswordErrorCode(t *testing.T) {
	if passwordErrorCode(domain.ErrPasswordTooLong) != codePasswordTooLong {
		t.Error("senha longa")
	}
	if passwordErrorCode(domain.ErrPasswordTooWeak) != codePasswordTooShort {
		t.Error("senha curta")
	}
	if passwordErrorCode(errors.New("outro")) != codePasswordTooShort {
		t.Error("erro desconhecido cai em senha curta")
	}
}

// O limite por IP responde com código próprio, distinto do reenvio cedo demais.
func TestRateLimiterAnswersWithCode(t *testing.T) {
	rl := newRateLimiter(1, time.Minute)
	h := rl.wrap(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	for i, want := range []int{http.StatusOK, http.StatusTooManyRequests} {
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil))
		if rec.Code != want {
			t.Fatalf("pedido %d: status %d", i, rec.Code)
		}
		if want == http.StatusTooManyRequests {
			if decodeAPIError(t, rec).Code != codeRateLimited || rec.Header().Get("Retry-After") == "" {
				t.Error("429 sem código ou sem Retry-After")
			}
		}
	}
}

// Rotas que recusam antes de tocar no banco: o corpo traz o código certo.
func TestHandlersAnswerWithCodes(t *testing.T) {
	s := &Server{}
	cases := []struct {
		name    string
		handler http.HandlerFunc
		body    string
		status  int
		code    string
	}{
		{"login com JSON quebrado", s.loginHandler, `{`, 400, codeInvalidRequest},
		{"login com senha gigante", s.loginHandler, `{"email":"a@x.com","password":"` + string(bytes.Repeat([]byte("a"), 600)) + `"}`, 401, codeInvalidCredentials},
		{"cadastro com e-mail inválido", s.registerHandler, `{"email":"nao-e-email"}`, 400, codeInvalidEmail},
		{"cadastro com senha curta", s.registerHandler, `{"email":"a@x.com","password":"123"}`, 400, codePasswordTooShort},
		{"cadastro com país inválido", s.registerHandler, `{"email":"a@x.com","password":"senha-forte","country":"XYZ"}`, 400, codeInvalidCountry},
		{"cadastro sem idade", s.registerHandler, `{"email":"a@x.com","password":"senha-forte","country":"BR"}`, 400, codeAgeNotConfirmed},
		{"código com e-mail inválido", s.requestOTPHandler, `{"email":"x"}`, 400, codeInvalidEmail},
		{"verificação com e-mail inválido", s.verifyOTPHandler, `{"email":"x","code":"1"}`, 401, codeOTPInvalid},
		{"troca de senha com senha longa", s.resetPasswordHandler, `{"email":"a@x.com","password":"` + string(bytes.Repeat([]byte("a"), 200)) + `"}`, 400, codePasswordTooLong},
		{"sync sem token", s.syncHandler, `{}`, 401, codeUnauthenticated},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		c.handler(rec, httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(c.body)))
		if rec.Code != c.status {
			t.Errorf("%s: status %d, want %d (%s)", c.name, rec.Code, c.status, rec.Body.String())
			continue
		}
		if got := decodeAPIError(t, rec).Code; got != c.code {
			t.Errorf("%s: code %q, want %q", c.name, got, c.code)
		}
	}
}
