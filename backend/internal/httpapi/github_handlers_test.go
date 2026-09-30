package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/josecleiton/logn/backend/internal/infrastructure/socialauth"
)

// fakeExchanger troca só o código que conhece. Guarda se a troca pediu para manter o
// token, que é o que separa login de exclusão.
type fakeExchanger struct {
	err       error
	keptToken []bool
}

func (f *fakeExchanger) Exchange(_ context.Context, code, _, _ string, keepToken bool) (socialauth.Exchanged, error) {
	f.keptToken = append(f.keptToken, keepToken)
	if code != "codigo-bom" {
		return socialauth.Exchanged{}, errors.New("bad_verification_code")
	}
	out := socialauth.Exchanged{Ticket: "bilhete"}
	if keepToken {
		out.AccessToken = "gho_da_exclusao"
	}
	return out, f.err
}

func exchange(s *Server, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	s.githubExchangeHandler(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
	return rec
}

func TestGitHubExchangeRefuses(t *testing.T) {
	enabled := &Server{github: &fakeExchanger{}}
	cases := []struct {
		name   string
		s      *Server
		body   string
		status int
		code   string
	}{
		{"GitHub desligado", &Server{}, `{"code":"codigo-bom","purpose":"login"}`, 503, codeProviderDisabled},
		{"JSON quebrado", enabled, `{`, 400, codeInvalidRequest},
		{"sem propósito", enabled, `{"code":"codigo-bom"}`, 400, codeInvalidRequest},
		{"propósito desconhecido", enabled, `{"code":"codigo-bom","purpose":"admin"}`, 400, codeInvalidRequest},
		{"código recusado", enabled, `{"code":"forjado","purpose":"login"}`, 401, codeSocialTokenInvalid},
		// Sem sessão, a troca de exclusão só devolveria um token vivo do GitHub.
		{"exclusão sem sessão", enabled, `{"code":"codigo-bom","purpose":"delete"}`, 401, codeUnauthenticated},
	}
	for _, c := range cases {
		rec := exchange(c.s, c.body)
		if rec.Code != c.status || decodeAPIError(t, rec).Code != c.code {
			t.Errorf("%s: %d %s, want %d %s", c.name, rec.Code, rec.Body.String(), c.status, c.code)
		}
		// O motivo do GitHub fica no log: dizer qual checagem falhou ensina a próxima.
		if strings.Contains(rec.Body.String(), "bad_verification_code") {
			t.Errorf("%s: resposta ecoa o motivo do GitHub: %s", c.name, rec.Body.String())
		}
	}
}

func TestGitHubExchangeKeepsTheTokenOnlyToDelete(t *testing.T) {
	f := newSocialFixture(t, fakeVerifier{})
	ex := &fakeExchanger{}
	f.s.github = ex
	s := f.s
	userID := f.seedPasswordUser(t, f.email("gh-troca"))

	var login GitHubExchangeResponse
	rec := exchange(s, `{"code":"codigo-bom","code_verifier":"v","nonce_hash":"h","purpose":"login"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("login: %d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("o bilhete não pode ficar em cache")
	}
	json.Unmarshal(rec.Body.Bytes(), &login)
	if login.Ticket != "bilhete" || login.AccessToken != "" {
		t.Fatalf("login devolve só o bilhete: %+v", login)
	}

	var del GitHubExchangeResponse
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"code":"codigo-bom","code_verifier":"v","nonce_hash":"h","purpose":"delete"}`))
	req.Header.Set("Authorization", bearer(t, userID))
	rec = httptest.NewRecorder()
	s.githubExchangeHandler(rec, req)
	json.Unmarshal(rec.Body.Bytes(), &del)
	if del.Ticket != "bilhete" || del.AccessToken != "gho_da_exclusao" {
		t.Fatalf("exclusão devolve bilhete e token: %+v", del)
	}
	if len(ex.keptToken) != 2 || ex.keptToken[0] || !ex.keptToken[1] {
		t.Fatalf("só a exclusão mantém o token: %v", ex.keptToken)
	}
}

// O token que ficou vivo no GitHub não segura o login: o bilhete já vale.
func TestGitHubExchangeTokenNotDeletedStillLogsIn(t *testing.T) {
	s := &Server{github: &fakeExchanger{err: socialauth.ErrTokenNotDeleted}}
	rec := exchange(s, `{"code":"codigo-bom","purpose":"login"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "bilhete") {
		t.Fatalf("login com token não apagado: %d %s", rec.Code, rec.Body.String())
	}
}

func TestGitHubExchangeBodyOverTheLimit(t *testing.T) {
	s := &Server{github: &fakeExchanger{}}
	handler := newRateLimiter(1000, 60e9).wrap(limitBody(authBodyLimit, s.githubExchangeHandler))
	body := `{"purpose":"login","code":"` + strings.Repeat("a", authBodyLimit) + `"}`
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
	if rec.Code != http.StatusBadRequest || decodeAPIError(t, rec).Code != codeInvalidRequest {
		t.Fatalf("corpo acima do limite: %d %s", rec.Code, rec.Body.String())
	}
}

func TestGitHubExchangeIsRateLimited(t *testing.T) {
	s := &Server{github: &fakeExchanger{}}
	handler := newRateLimiter(2, 60e9).wrap(limitBody(authBodyLimit, s.githubExchangeHandler))
	var last int
	for range 3 {
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"code":"forjado","purpose":"login"}`)))
		last = rec.Code
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("a terceira troca no minuto tinha de ser barrada, veio %d", last)
	}
}

// fakeGitHubRevoker é o revogador do GitHub no teste: além de revogar, guarda os tokens
// descartados de exclusões que não aconteceram.
type fakeGitHubRevoker struct {
	fakeRevoker
	discarded []string
}

func (f *fakeGitHubRevoker) Discard(_ context.Context, token string) error {
	f.discarded = append(f.discarded, token)
	return nil
}

// withGitHub liga o GitHub na fixture: os bilhetes que o verificador conhece e o
// revogador falso.
func (f *socialFixture) withGitHub(tickets fakeVerifier) *fakeGitHubRevoker {
	rv := &fakeGitHubRevoker{}
	f.s.social[socialauth.ProviderGitHub] = tickets
	if f.s.revokers == nil {
		f.s.revokers = map[string]socialauth.Revoker{}
	}
	f.s.revokers[socialauth.ProviderGitHub] = rv
	return rv
}

func TestGitHubSignupAndDeletionRevokesTheGrant(t *testing.T) {
	f := newSocialFixture(t, fakeVerifier{})
	github := fakeVerifier{}
	rv := f.withGitHub(github)
	addr := f.email("github")
	f.cleanupEmail(t, addr)
	sub := "583231" + f.tag
	github["bilhete"] = socialauth.Identity{Subject: sub, Email: addr, EmailVerified: true}

	body := f.signupBody("bilhete")
	body["provider"] = "github"
	userID := sessionUser(t, f.post(t, body))
	if owns, _ := f.s.repo.HasIdentity(context.Background(), userID, "github", sub); !owns {
		t.Fatal("identidade do GitHub não gravada")
	}

	// Sem o access token não há como revogar: a exclusão pelo GitHub não passa.
	expect(t, f.deleteAs(t, userID, `{"provider":"github","id_token":"bilhete","nonce":"`+fakeNonce+`"}`),
		http.StatusBadRequest, codeInvalidRequest)
	if !f.s.repo.IsUserActive(context.Background(), userID) {
		t.Fatal("conta desativada sem revogação possível")
	}

	rec := f.deleteAs(t, userID, `{"provider":"github","id_token":"bilhete","nonce":"`+fakeNonce+`","authorization_code":"gho_da_exclusao"}`)
	if rec.Code != http.StatusOK || f.s.repo.IsUserActive(context.Background(), userID) {
		t.Fatalf("exclusão pelo GitHub: %d %s", rec.Code, rec.Body.String())
	}
	if want := "gho_da_exclusao@" + sub; len(rv.codes) != 1 || rv.codes[0] != want {
		t.Fatalf("revogação recebeu %v, want [%s]", rv.codes, want)
	}
	if len(rv.discarded) != 0 {
		t.Fatalf("o token da exclusão que deu certo vai para a revogação, não para o descarte: %v", rv.discarded)
	}
}

// Login no GitHub de mais de 5 minutos não prova ninguém na frente da tela, e o token
// que veio com ele não fica vivo.
func TestGitHubStaleTicketDoesNotDelete(t *testing.T) {
	f := newSocialFixture(t, fakeVerifier{})
	github := fakeVerifier{}
	rv := f.withGitHub(github)
	addr := f.email("gh-velho")
	f.cleanupEmail(t, addr)
	sub := "gh-velho-" + f.tag
	github["novo"] = socialauth.Identity{Subject: sub, Email: addr, EmailVerified: true}
	body := f.signupBody("novo")
	body["provider"] = "github"
	userID := sessionUser(t, f.post(t, body))

	github["velho"] = socialauth.Identity{Subject: sub, Email: addr, EmailVerified: true, IssuedAt: time.Now().Add(-6 * time.Minute)}
	expect(t, f.deleteAs(t, userID, `{"provider":"github","id_token":"velho","nonce":"`+fakeNonce+`","authorization_code":"gho_velho"}`),
		http.StatusUnauthorized, codeSocialTokenInvalid)
	if !f.s.repo.IsUserActive(context.Background(), userID) || len(rv.codes) != 0 {
		t.Fatal("bilhete velho apagou ou revogou")
	}
	if len(rv.discarded) != 1 || rv.discarded[0] != "gho_velho" {
		t.Fatalf("o token da exclusão recusada tinha de ser descartado: %v", rv.discarded)
	}
}

// Conta com Apple e GitHub: excluir pelo GitHub cai no 409 da Apple, e o token do
// GitHub que a troca devolveu não fica vivo.
func TestGitHubDeletionRefusedForAppleDiscardsTheToken(t *testing.T) {
	f := newSocialFixture(t, fakeVerifier{})
	apple := fakeVerifier{}
	appleRv := f.withApple(apple)
	github := fakeVerifier{}
	rv := f.withGitHub(github)
	addr := f.email("apple-e-github")
	f.cleanupEmail(t, addr)
	apple["a"] = socialauth.Identity{Subject: "apple-gh-" + f.tag, Email: addr, EmailVerified: true}
	github["g"] = socialauth.Identity{Subject: "gh-apple-" + f.tag, Email: addr, EmailVerified: true}
	userID := sessionUser(t, f.post(t, f.appleBody("a")))
	gh := loginBody("g")
	gh["provider"] = "github"
	if got := sessionUser(t, f.post(t, gh)); got != userID {
		t.Fatalf("GitHub entrou em %s, want %s", got, userID)
	}

	expect(t, f.deleteAs(t, userID, `{"provider":"github","id_token":"g","nonce":"`+fakeNonce+`","authorization_code":"gho_x"}`),
		http.StatusConflict, codeProviderReauthRequired)
	if !f.s.repo.IsUserActive(context.Background(), userID) || len(appleRv.codes) != 0 || len(rv.codes) != 0 {
		t.Fatal("exclusão pelo GitHub passou sem a Apple")
	}
	if len(rv.discarded) != 1 || rv.discarded[0] != "gho_x" {
		t.Fatalf("o token do GitHub tinha de ser descartado: %v", rv.discarded)
	}
}

// O bilhete de outra conta do GitHub não apaga esta.
func TestGitHubDeletionNeedsItsOwnIdentity(t *testing.T) {
	f := newSocialFixture(t, fakeVerifier{})
	github := fakeVerifier{}
	rv := f.withGitHub(github)
	mine, theirs := f.email("gh-dono"), f.email("gh-outro")
	f.cleanupEmail(t, mine)
	f.cleanupEmail(t, theirs)
	github["meu"] = socialauth.Identity{Subject: "gh-dono-" + f.tag, Email: mine, EmailVerified: true}
	github["dele"] = socialauth.Identity{Subject: "gh-outro-" + f.tag, Email: theirs, EmailVerified: true}

	signup := func(ticket string) string {
		b := f.signupBody(ticket)
		b["provider"] = "github"
		return sessionUser(t, f.post(t, b))
	}
	userID := signup("meu")
	signup("dele")

	expect(t, f.deleteAs(t, userID, `{"provider":"github","id_token":"dele","nonce":"`+fakeNonce+`","authorization_code":"x"}`),
		http.StatusUnauthorized, codeInvalidCredentials)
	if !f.s.repo.IsUserActive(context.Background(), userID) || len(rv.codes) != 0 {
		t.Fatal("bilhete de outra conta apagou ou revogou")
	}
}

// E-mail principal não verificado no GitHub não liga nem cria conta.
func TestGitHubUnverifiedPrimaryEmailIsRefused(t *testing.T) {
	f := newSocialFixture(t, fakeVerifier{})
	github := fakeVerifier{}
	f.withGitHub(github)
	addr := f.email("gh-nao-verificado")
	f.seedPasswordUser(t, addr)
	github["bilhete"] = socialauth.Identity{Subject: "gh-nv-" + f.tag, Email: addr, EmailVerified: false}

	body := loginBody("bilhete")
	body["provider"] = "github"
	expect(t, f.post(t, body), http.StatusForbidden, codeSocialEmailUnverified)
}

// O GitHub não exige revogar: a conta com senha e GitHub ligado sai pela senha.
func TestPasswordDeletionOfAGitHubLinkedAccount(t *testing.T) {
	f := newSocialFixture(t, fakeVerifier{})
	f.withGitHub(fakeVerifier{})
	userID := f.seedPasswordUser(t, f.email("senha-e-github"))
	if _, err := f.s.repo.LinkIdentity(context.Background(), "github", "gh-"+f.tag, userID); err != nil {
		t.Fatal(err)
	}
	if rec := f.deleteAs(t, userID, `{"password":"senha-forte-do-teste"}`); rec.Code != http.StatusOK {
		t.Fatalf("conta com GitHub barrada na senha: %d %s", rec.Code, rec.Body.String())
	}
}
