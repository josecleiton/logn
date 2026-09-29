package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/josecleiton/logn/backend/internal/domain"
	"github.com/josecleiton/logn/backend/internal/infrastructure/email"
	"github.com/josecleiton/logn/backend/internal/infrastructure/socialauth"
)

// fakeVerifier aceita os tokens que conhece, cada um com a identidade dele e um nonce
// fixo. Qualquer outro token, ou o nonce errado, é recusado: é o que o de verdade faz
// com assinatura, audiência, prazo ou nonce que não conferem.
type fakeVerifier map[string]socialauth.Identity

const fakeNonce = "nonce-do-teste"

func (f fakeVerifier) Verify(_ context.Context, idToken, nonce string) (socialauth.Identity, error) {
	id, ok := f[idToken]
	if !ok || nonce != fakeNonce {
		return socialauth.Identity{}, errors.New("recusado")
	}
	if id.IssuedAt.IsZero() {
		id.IssuedAt = time.Now()
	}
	return id, nil
}

type socialFixture struct {
	s    *Server
	pool *pgxpool.Pool
	// Um sufixo por teste: os e-mails não colidem com os de outra execução.
	tag string
}

func newSocialFixture(t *testing.T, tokens fakeVerifier) *socialFixture {
	t.Helper()
	pool := setupTestDB(t)
	domain.JwtSecretKey = []byte("test-secret")
	f := &socialFixture{
		s: &Server{
			repo:   domain.NewRepository(pool),
			mailer: email.NewMailer(),
			social: map[string]socialauth.Verifier{socialauth.ProviderGoogle: tokens},
		},
		pool: pool,
		tag:  newTestUUID(t)[:8],
	}
	return f
}

func (f *socialFixture) email(name string) string {
	return name + "-" + f.tag + "@example.com"
}

// cleanupEmail apaga a conta criada pelo teste; a identidade sai em cascata.
func (f *socialFixture) cleanupEmail(t *testing.T, addr string) {
	t.Cleanup(func() {
		f.pool.Exec(context.Background(), `DELETE FROM users WHERE email = $1`, addr)
	})
}

func (f *socialFixture) seedPasswordUser(t *testing.T, addr string) string {
	t.Helper()
	hash, err := domain.HashPassword("senha-forte-do-teste")
	if err != nil {
		t.Fatal(err)
	}
	id, err := f.s.repo.CreateUser(context.Background(), addr, hash, true, "BR", nil)
	if err != nil {
		t.Fatalf("usuário: %v", err)
	}
	f.cleanupEmail(t, addr)
	return id
}

func (f *socialFixture) post(t *testing.T, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	f.s.socialLoginHandler(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/social", bytes.NewReader(raw)))
	return rec
}

func loginBody(token string) map[string]any {
	return map[string]any{"provider": "google", "id_token": token, "nonce": fakeNonce}
}

func signupBody(token string) map[string]any {
	b := loginBody(token)
	b["age_confirmed"] = true
	b["country"] = "BR"
	b["legal_acceptances"] = []map[string]any{
		{"kind": "terms", "version": 1, "locale": "pt-BR"},
		{"kind": "privacy", "version": 1, "locale": "pt-BR"},
	}
	return b
}

func sessionUser(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var resp AuthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.AccessToken == "" || resp.RefreshToken == "" || resp.UserID == "" || resp.Email == "" {
		t.Fatalf("sessão incompleta: %s", rec.Body.String())
	}
	return resp.UserID
}

func TestSocialLoginRefusesBeforeTouchingAccounts(t *testing.T) {
	disabled := &Server{social: map[string]socialauth.Verifier{}}
	cases := []struct {
		name   string
		s      *Server
		body   string
		status int
		code   string
	}{
		{"JSON quebrado", disabled, `{`, 400, codeInvalidRequest},
		{"Google desligado", disabled, `{"provider":"google","id_token":"t","nonce":"n"}`, 503, codeProviderDisabled},
		{"provedor desconhecido", disabled, `{"provider":"myspace","id_token":"t","nonce":"n"}`, 400, codeInvalidRequest},
		{"token recusado", &Server{social: map[string]socialauth.Verifier{"google": fakeVerifier{}}},
			`{"provider":"google","id_token":"forjado","nonce":"` + fakeNonce + `"}`, 401, codeSocialTokenInvalid},
		{"nonce de outro login", &Server{social: map[string]socialauth.Verifier{"google": fakeVerifier{"bom": {Subject: "s"}}}},
			`{"provider":"google","id_token":"bom","nonce":"outro"}`, 401, codeSocialTokenInvalid},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		c.s.socialLoginHandler(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(c.body)))
		if rec.Code != c.status || decodeAPIError(t, rec).Code != c.code {
			t.Errorf("%s: %d %s, want %d %s", c.name, rec.Code, rec.Body.String(), c.status, c.code)
		}
	}
}

func TestSocialLoginBodyOverTheLimit(t *testing.T) {
	s := &Server{social: map[string]socialauth.Verifier{"google": fakeVerifier{}}}
	handler := newRateLimiter(1000, 60e9).wrap(limitBody(authBodyLimit, s.socialLoginHandler))
	body := `{"provider":"google","nonce":"n","id_token":"` + strings.Repeat("a", authBodyLimit) + `"}`
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
	if rec.Code != http.StatusBadRequest || decodeAPIError(t, rec).Code != codeInvalidRequest {
		t.Fatalf("corpo acima do limite: %d %s", rec.Code, rec.Body.String())
	}
}

func TestSocialSignupNeedsAgeAndTerms(t *testing.T) {
	tokens := fakeVerifier{}
	f := newSocialFixture(t, tokens)
	addr := f.email("nova")
	f.cleanupEmail(t, addr)
	tokens["novo"] = socialauth.Identity{Subject: "sub-novo-" + f.tag, Email: addr, EmailVerified: true}

	// Sem nada do cadastro: o app ainda tem de pedir idade e aceite.
	rec := f.post(t, loginBody("novo"))
	expect(t, rec, http.StatusConflict, codeSignupRequired)

	noAge := signupBody("novo")
	noAge["age_confirmed"] = false
	expect(t, f.post(t, noAge), http.StatusBadRequest, codeAgeNotConfirmed)

	noTerms := signupBody("novo")
	noTerms["legal_acceptances"] = []map[string]any{{"kind": "terms", "version": 1, "locale": "pt-BR"}}
	expect(t, f.post(t, noTerms), http.StatusBadRequest, codeLegalAcceptanceRequired)

	oldTerms := signupBody("novo")
	oldTerms["legal_acceptances"] = []map[string]any{
		{"kind": "terms", "version": 0, "locale": "pt-BR"},
		{"kind": "privacy", "version": 1, "locale": "pt-BR"},
	}
	expect(t, f.post(t, oldTerms), http.StatusConflict, codeLegalVersionOutdated)

	badCountry := signupBody("novo")
	badCountry["country"] = "XYZ"
	expect(t, f.post(t, badCountry), http.StatusBadRequest, codeInvalidCountry)

	// Nenhuma das tentativas recusadas deixou conta para trás.
	var n int
	f.pool.QueryRow(context.Background(), `SELECT count(*) FROM users WHERE email = $1`, addr).Scan(&n)
	if n != 0 {
		t.Fatalf("conta criada por pedido recusado: %d", n)
	}

	userID := sessionUser(t, f.post(t, signupBody("novo")))

	// Conta sem senha, com idade e aceites gravados.
	var hasPassword, ageConfirmed bool
	var acceptances int
	f.pool.QueryRow(context.Background(), `
		SELECT password_hash IS NOT NULL, age_confirmed_at IS NOT NULL,
		       (SELECT count(*) FROM legal_acceptances WHERE user_id = u.id)
		FROM users u WHERE id = $1`, userID).Scan(&hasPassword, &ageConfirmed, &acceptances)
	if hasPassword || !ageConfirmed || acceptances != 2 {
		t.Fatalf("conta nova: senha=%v idade=%v aceites=%d", hasPassword, ageConfirmed, acceptances)
	}

	// Da segunda vez, o `sub` basta.
	if again := sessionUser(t, f.post(t, loginBody("novo"))); again != userID {
		t.Fatalf("segundo login em outra conta: %s, want %s", again, userID)
	}

	// A conta sem senha não entra pela senha, nem pela vazia.
	rec = httptest.NewRecorder()
	f.s.loginHandler(rec, httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader(`{"email":"`+addr+`","password":""}`)))
	expect(t, rec, http.StatusUnauthorized, codeInvalidCredentials)
}

func TestSocialLoginRefusesUnverifiedEmail(t *testing.T) {
	tokens := fakeVerifier{}
	f := newSocialFixture(t, tokens)
	addr := f.email("vitima")
	victim := f.seedPasswordUser(t, addr)
	tokens["forjado"] = socialauth.Identity{Subject: "sub-atacante-" + f.tag, Email: addr, EmailVerified: false}

	expect(t, f.post(t, signupBody("forjado")), http.StatusForbidden, codeSocialEmailUnverified)

	owns, _ := f.s.repo.HasIdentity(context.Background(), victim, "google", "sub-atacante-"+f.tag)
	if owns {
		t.Fatal("identidade com e-mail não verificado ligada à conta de outra pessoa")
	}
}

func TestSocialLoginLinksTheExistingAccount(t *testing.T) {
	tokens := fakeVerifier{}
	f := newSocialFixture(t, tokens)
	addr := f.email("antiga")
	existing := f.seedPasswordUser(t, addr)
	sub := "sub-antiga-" + f.tag
	// O provedor manda o e-mail com maiúscula; a conta foi gravada normalizada.
	tokens["antiga"] = socialauth.Identity{Subject: sub, Email: strings.ToUpper(addr), EmailVerified: true}

	if got := sessionUser(t, f.post(t, loginBody("antiga"))); got != existing {
		t.Fatalf("entrou em %s, want a conta existente %s", got, existing)
	}
	if owns, _ := f.s.repo.HasIdentity(context.Background(), existing, "google", sub); !owns {
		t.Fatal("identidade não ficou ligada")
	}

	// O e-mail mudou no provedor; o `sub` continua levando à mesma conta, e a sessão
	// segue com o e-mail da conta, não o novo do provedor.
	tokens["antiga-email-novo"] = socialauth.Identity{Subject: sub, Email: f.email("trocado"), EmailVerified: true}
	rec := f.post(t, loginBody("antiga-email-novo"))
	if got := sessionUser(t, rec); got != existing {
		t.Fatalf("e-mail trocado levou a %s, want %s", got, existing)
	}
	var resp AuthResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Email != addr {
		t.Fatalf("sessão com e-mail %q, want o da conta %q", resp.Email, addr)
	}

	// A senha continua valendo depois do vínculo.
	rec = httptest.NewRecorder()
	f.s.loginHandler(rec, httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader(`{"email":"`+addr+`","password":"senha-forte-do-teste"}`)))
	sessionUser(t, rec)
}

func TestSocialLoginSubjectWinsOverEmail(t *testing.T) {
	tokens := fakeVerifier{}
	f := newSocialFixture(t, tokens)
	a := f.seedPasswordUser(t, f.email("a"))
	b := f.seedPasswordUser(t, f.email("b"))
	sub := "sub-de-a-" + f.tag
	if _, err := f.s.repo.LinkIdentity(context.Background(), "google", sub, a); err != nil {
		t.Fatal(err)
	}

	// O `sub` já é de A; o e-mail que o token traz agora é o de B.
	tokens["t"] = socialauth.Identity{Subject: sub, Email: f.email("b"), EmailVerified: true}
	if got := sessionUser(t, f.post(t, loginBody("t"))); got != a {
		t.Fatalf("entrou em %s, want A (%s); B é %s", got, a, b)
	}
	if owns, _ := f.s.repo.HasIdentity(context.Background(), b, "google", sub); owns {
		t.Fatal("identidade de A ligada a B")
	}
}

func TestSocialLoginCancelsAPendingDeletion(t *testing.T) {
	tokens := fakeVerifier{}
	f := newSocialFixture(t, tokens)
	addr := f.email("excluindo")
	userID := f.seedPasswordUser(t, addr)
	if _, err := f.s.repo.MarkAccountForDeletion(context.Background(), userID); err != nil {
		t.Fatal(err)
	}
	tokens["t"] = socialauth.Identity{Subject: "sub-excluindo-" + f.tag, Email: addr, EmailVerified: true}

	rec := f.post(t, loginBody("t"))
	sessionUser(t, rec)
	var resp AuthResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.AccountRestored || !f.s.repo.IsUserActive(context.Background(), userID) {
		t.Fatalf("exclusão não cancelada: %s", rec.Body.String())
	}
}

func TestDeleteAccountWithPasswordRefusesAnUnlinkedIdentity(t *testing.T) {
	tokens := fakeVerifier{}
	f := newSocialFixture(t, tokens)
	addr := f.email("com-senha")
	userID := f.seedPasswordUser(t, addr)
	// Conta Google válida, com o mesmo e-mail, mas nunca ligada a esta conta.
	tokens["solta"] = socialauth.Identity{Subject: "sub-solta-" + f.tag, Email: addr, EmailVerified: true}

	req := httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader(`{"provider":"google","id_token":"solta","nonce":"`+fakeNonce+`"}`))
	req.Header.Set("Authorization", bearer(t, userID))
	rec := httptest.NewRecorder()
	f.s.deleteAccountHandler(rec, req)
	expect(t, rec, http.StatusUnauthorized, codeInvalidCredentials)
	if !f.s.repo.IsUserActive(context.Background(), userID) {
		t.Fatal("conta desativada por identidade que não é dela")
	}
}

// A conta sem senha ganha uma pelo código do e-mail, como quem esqueceu a senha: o
// código prova o mesmo e-mail que o provedor provou.
func TestPasswordlessAccountCanSetAPasswordByEmailCode(t *testing.T) {
	tokens := fakeVerifier{}
	f := newSocialFixture(t, tokens)
	addr := f.email("ganha-senha")
	f.cleanupEmail(t, addr)
	tokens["t"] = socialauth.Identity{Subject: "sub-ganha-" + f.tag, Email: addr, EmailVerified: true}
	userID := sessionUser(t, f.post(t, signupBody("t")))

	const code = "123456"
	if err := f.s.repo.SaveOTP(context.Background(), addr, code, domain.OTPPurposeResetPassword, time.Minute); err != nil {
		t.Fatalf("código: %v", err)
	}
	t.Cleanup(func() { f.pool.Exec(context.Background(), `DELETE FROM otps WHERE email = $1`, addr) })

	rec := httptest.NewRecorder()
	f.s.resetPasswordHandler(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(
		`{"email":"`+addr+`","otp":"`+code+`","password":"senha-nova-forte"}`)))
	if got := sessionUser(t, rec); got != userID {
		t.Fatalf("troca de senha em %s, want %s", got, userID)
	}
	// E o Google continua entrando na mesma conta.
	if got := sessionUser(t, f.post(t, loginBody("t"))); got != userID {
		t.Fatalf("login social depois da senha: %s, want %s", got, userID)
	}
}

func TestDeleteAccountWithoutPasswordNeedsItsOwnIdentity(t *testing.T) {
	tokens := fakeVerifier{}
	f := newSocialFixture(t, tokens)
	addr := f.email("sem-senha")
	f.cleanupEmail(t, addr)
	tokens["dono"] = socialauth.Identity{Subject: "sub-dono-" + f.tag, Email: addr, EmailVerified: true}
	tokens["outro"] = socialauth.Identity{Subject: "sub-outro-" + f.tag, Email: f.email("outro"), EmailVerified: true}
	userID := sessionUser(t, f.post(t, signupBody("dono")))

	del := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req.Header.Set("Authorization", bearer(t, userID))
		rec := httptest.NewRecorder()
		f.s.deleteAccountHandler(rec, req)
		return rec
	}

	// Conta sem senha: senha nenhuma serve, nem a vazia.
	expect(t, del(`{"password":""}`), http.StatusUnauthorized, codeInvalidCredentials)
	// Login válido de outra conta Google não apaga esta.
	expect(t, del(`{"provider":"google","id_token":"outro","nonce":"`+fakeNonce+`"}`), http.StatusUnauthorized, codeInvalidCredentials)
	// Token da própria conta com nonce errado é token recusado.
	expect(t, del(`{"provider":"google","id_token":"dono","nonce":"x"}`), http.StatusUnauthorized, codeSocialTokenInvalid)
	// Login da própria conta, mas antigo: excluir pede login de agora.
	stale := tokens["dono"]
	stale.IssuedAt = time.Now().Add(-deleteReauthMaxAge - time.Minute)
	tokens["dono-antigo"] = stale
	expect(t, del(`{"provider":"google","id_token":"dono-antigo","nonce":"`+fakeNonce+`"}`), http.StatusUnauthorized, codeSocialTokenInvalid)
	if !f.s.repo.IsUserActive(context.Background(), userID) {
		t.Fatal("conta desativada por pedido recusado")
	}

	rec := del(`{"provider":"google","id_token":"dono","nonce":"` + fakeNonce + `"}`)
	if rec.Code != http.StatusOK || f.s.repo.IsUserActive(context.Background(), userID) {
		t.Fatalf("exclusão com a própria identidade: %d %s", rec.Code, rec.Body.String())
	}
}

// fakeRevoker guarda os códigos que recebeu, com o `sub` de cada um, e falha quando
// mandado.
type fakeRevoker struct {
	codes []string
	fail  bool
}

func (f *fakeRevoker) Revoke(_ context.Context, code, subject string) error {
	f.codes = append(f.codes, code+"@"+subject)
	if f.fail {
		return errors.New("apple fora do ar")
	}
	return nil
}

// withApple liga a Apple na fixture, com os tokens dela e o revogador falso.
func (f *socialFixture) withApple(tokens fakeVerifier) *fakeRevoker {
	rv := &fakeRevoker{}
	f.s.social[socialauth.ProviderApple] = tokens
	f.s.revokers = map[string]socialauth.Revoker{socialauth.ProviderApple: rv}
	return rv
}

func appleBody(token string) map[string]any {
	b := signupBody(token)
	b["provider"] = "apple"
	return b
}

func (f *socialFixture) deleteAs(t *testing.T, userID, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Authorization", bearer(t, userID))
	rec := httptest.NewRecorder()
	f.s.deleteAccountHandler(rec, req)
	return rec
}

func TestAppleDisabledWithoutItsKey(t *testing.T) {
	s := &Server{social: map[string]socialauth.Verifier{}}
	rec := httptest.NewRecorder()
	s.socialLoginHandler(rec, httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader(`{"provider":"apple","id_token":"t","nonce":"n"}`)))
	expect(t, rec, http.StatusServiceUnavailable, codeProviderDisabled)
}

func TestAppleSignupAndDeletionRevokesTheGrant(t *testing.T) {
	f := newSocialFixture(t, fakeVerifier{})
	apple := fakeVerifier{}
	rv := f.withApple(apple)
	addr := f.email("apple")
	f.cleanupEmail(t, addr)
	apple["t"] = socialauth.Identity{Subject: "apple-sub-" + f.tag, Email: addr, EmailVerified: true}

	userID := sessionUser(t, f.post(t, appleBody("t")))
	if owns, _ := f.s.repo.HasIdentity(context.Background(), userID, "apple", "apple-sub-"+f.tag); !owns {
		t.Fatal("identidade da Apple não gravada")
	}

	// Sem o código, não há como revogar: a exclusão não passa.
	expect(t, f.deleteAs(t, userID, `{"provider":"apple","id_token":"t","nonce":"`+fakeNonce+`"}`),
		http.StatusBadRequest, codeInvalidRequest)
	if !f.s.repo.IsUserActive(context.Background(), userID) {
		t.Fatal("conta desativada sem revogação possível")
	}

	rec := f.deleteAs(t, userID, `{"provider":"apple","id_token":"t","nonce":"`+fakeNonce+`","authorization_code":"code-1"}`)
	if rec.Code != http.StatusOK || f.s.repo.IsUserActive(context.Background(), userID) {
		t.Fatalf("exclusão pela Apple: %d %s", rec.Code, rec.Body.String())
	}
	// O código vai com o `sub` que provou a posse: é ele que a revogação confere.
	if want := "code-1@apple-sub-" + f.tag; len(rv.codes) != 1 || rv.codes[0] != want {
		t.Fatalf("revogação recebeu %v, want [%s]", rv.codes, want)
	}
}

// Conta com Google e Apple: confirmar pelo Google não basta, porque a Apple ficaria
// autorizada.
func TestGoogleDeletionOfAnAppleLinkedAccountGoesThroughApple(t *testing.T) {
	google := fakeVerifier{}
	f := newSocialFixture(t, google)
	apple := fakeVerifier{}
	rv := f.withApple(apple)
	addr := f.email("dois-provedores")
	f.cleanupEmail(t, addr)
	apple["a"] = socialauth.Identity{Subject: "apple-dois-" + f.tag, Email: addr, EmailVerified: true}
	google["g"] = socialauth.Identity{Subject: "google-dois-" + f.tag, Email: addr, EmailVerified: true}
	userID := sessionUser(t, f.post(t, appleBody("a")))
	if got := sessionUser(t, f.post(t, loginBody("g"))); got != userID {
		t.Fatalf("Google entrou em %s, want %s", got, userID)
	}

	expect(t, f.deleteAs(t, userID, `{"provider":"google","id_token":"g","nonce":"`+fakeNonce+`"}`),
		http.StatusConflict, codeProviderReauthRequired)
	if !f.s.repo.IsUserActive(context.Background(), userID) || len(rv.codes) != 0 {
		t.Fatal("exclusão pelo Google passou sem revogar a Apple")
	}
}

// Apple desligada depois de haver contas nela: a exclusão pela senha não fica sem
// saída.
func TestDeletionWithAppleDisabledDoesNotTrapTheAccount(t *testing.T) {
	f := newSocialFixture(t, fakeVerifier{})
	addr := f.email("apple-desligada")
	userID := f.seedPasswordUser(t, addr)
	if _, err := f.s.repo.LinkIdentity(context.Background(), "apple", "apple-velha-"+f.tag, userID); err != nil {
		t.Fatal(err)
	}
	// Sem `withApple`: nenhum revogador ligado.
	if rec := f.deleteAs(t, userID, `{"password":"senha-forte-do-teste"}`); rec.Code != http.StatusOK {
		t.Fatalf("exclusão presa com a Apple desligada: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAppleOutageDoesNotBlockDeletion(t *testing.T) {
	f := newSocialFixture(t, fakeVerifier{})
	apple := fakeVerifier{}
	rv := f.withApple(apple)
	rv.fail = true
	addr := f.email("apple-fora")
	f.cleanupEmail(t, addr)
	apple["t"] = socialauth.Identity{Subject: "apple-fora-" + f.tag, Email: addr, EmailVerified: true}
	userID := sessionUser(t, f.post(t, appleBody("t")))

	rec := f.deleteAs(t, userID, `{"provider":"apple","id_token":"t","nonce":"`+fakeNonce+`","authorization_code":"c"}`)
	if rec.Code != http.StatusOK || f.s.repo.IsUserActive(context.Background(), userID) {
		t.Fatalf("Apple fora do ar segurou a exclusão: %d %s", rec.Code, rec.Body.String())
	}
}

func TestPasswordDeletionOfAnAppleLinkedAccountGoesThroughApple(t *testing.T) {
	f := newSocialFixture(t, fakeVerifier{})
	apple := fakeVerifier{}
	f.withApple(apple)
	addr := f.email("senha-e-apple")
	userID := f.seedPasswordUser(t, addr)
	apple["t"] = socialauth.Identity{Subject: "apple-ligada-" + f.tag, Email: addr, EmailVerified: true}
	// O primeiro login pela Apple liga a identidade à conta com senha.
	if got := sessionUser(t, f.post(t, appleBody("t"))); got != userID {
		t.Fatalf("login pela Apple em %s, want %s", got, userID)
	}

	expect(t, f.deleteAs(t, userID, `{"password":"senha-forte-do-teste"}`),
		http.StatusConflict, codeProviderReauthRequired)
	if !f.s.repo.IsUserActive(context.Background(), userID) {
		t.Fatal("conta desativada pela senha sem revogar a Apple")
	}

	// Conta com senha e só Google continua saindo pela senha.
	g := newSocialFixture(t, fakeVerifier{})
	g.withApple(fakeVerifier{})
	other := g.seedPasswordUser(t, g.email("so-google"))
	if _, err := g.s.repo.LinkIdentity(context.Background(), "google", "g-"+g.tag, other); err != nil {
		t.Fatal(err)
	}
	if rec := g.deleteAs(t, other, `{"password":"senha-forte-do-teste"}`); rec.Code != http.StatusOK {
		t.Fatalf("conta sem Apple barrada: %d %s", rec.Code, rec.Body.String())
	}
}
