package socialauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/josecleiton/logn/backend/internal/domain"
)

const (
	testGitHubClient = "Iv1.testclient"
	testGitHubSecret = "segredo-de-teste"
	testCode         = "codigo-do-github-0001"
	testVerifier     = "verifier-de-teste-com-quarenta-e-tres-letras-abc"
	testGitHubToken  = "gho_token_de_teste"
)

var testSessionKey = []byte("chave-da-sessao-de-teste")

// githubFake é o GitHub do teste: aceita o código e o verifier de teste, e diz quem é o
// dono do token. Guarda o que o servidor apagou e revogou.
type githubFake struct {
	server *httptest.Server
	g      *GitHub

	mu       sync.Mutex
	emails   []map[string]any
	userID   int64
	tokenApp string // client_id que a conferência do token devolve
	tokenErr string // `error` na troca do código
	deleted  []string
	revoked  []string
	failUser bool
}

func newGitHubFake(t *testing.T) *githubFake {
	t.Helper()
	f := &githubFake{
		userID:   583231,
		tokenApp: testGitHubClient,
		emails: []map[string]any{
			{"email": "secundario@example.com", "primary": false, "verified": true},
			{"email": "pessoa@example.com", "primary": true, "verified": true},
		},
	}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)
	g, err := NewGitHub(testGitHubClient, testGitHubSecret, "logn://oauth/github", testSessionKey, f.server.Client())
	if err != nil {
		t.Fatal(err)
	}
	g.oauthURL, g.apiURL = f.server.URL, f.server.URL
	f.g = g
	return f
}

func (f *githubFake) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	appAuth := func() bool {
		id, secret, ok := r.BasicAuth()
		return ok && id == testGitHubClient && secret == testGitHubSecret
	}
	bodyToken := func() string {
		var b struct {
			AccessToken string `json:"access_token"`
		}
		json.NewDecoder(r.Body).Decode(&b)
		return b.AccessToken
	}
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/login/oauth/access_token":
		r.ParseForm()
		if f.tokenErr != "" {
			json.NewEncoder(w).Encode(map[string]string{"error": f.tokenErr})
			return
		}
		if r.Form.Get("client_secret") != testGitHubSecret || r.Form.Get("code") != testCode ||
			r.Form.Get("code_verifier") != testVerifier || r.Form.Get("redirect_uri") != "logn://oauth/github" {
			json.NewEncoder(w).Encode(map[string]string{"error": "bad_verification_code"})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"access_token": testGitHubToken, "token_type": "bearer", "scope": "user:email"})
	case r.Method == http.MethodGet && r.URL.Path == "/user":
		if r.Header.Get("Authorization") != "Bearer "+testGitHubToken || f.failUser {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"id": f.userID, "login": "pessoa"})
	case r.Method == http.MethodGet && r.URL.Path == "/user/emails":
		if r.Header.Get("Authorization") != "Bearer "+testGitHubToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		json.NewEncoder(w).Encode(f.emails)
	case r.URL.Path == "/applications/"+testGitHubClient+"/token":
		if !appAuth() {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		tok := bodyToken()
		if r.Method == http.MethodDelete {
			f.deleted = append(f.deleted, tok)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"app":  map[string]string{"client_id": f.tokenApp},
			"user": map[string]any{"id": f.userID},
		})
	case r.Method == http.MethodDelete && r.URL.Path == "/applications/"+testGitHubClient+"/grant":
		if !appAuth() {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		f.revoked = append(f.revoked, bodyToken())
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func TestGitHubExchangeGivesATicketThatVerifies(t *testing.T) {
	f := newGitHubFake(t)
	out, err := f.g.Exchange(context.Background(), testCode, testVerifier, hashNonce(nonceA), false)
	if err != nil {
		t.Fatal(err)
	}
	if out.AccessToken != "" {
		t.Fatal("o login não devolve o access token")
	}
	if len(f.deleted) != 1 || f.deleted[0] != testGitHubToken {
		t.Fatalf("o token do login tem de ser apagado no GitHub, apagados: %v", f.deleted)
	}

	id, err := f.g.Verify(context.Background(), out.Ticket, nonceA)
	if err != nil {
		t.Fatal(err)
	}
	if id.Subject != "583231" || id.Email != "pessoa@example.com" || !id.EmailVerified {
		t.Fatalf("identidade errada: %+v", id)
	}
	if time.Since(id.IssuedAt) > time.Minute {
		t.Fatalf("iat do bilhete é a hora da troca: %v", id.IssuedAt)
	}
}

func TestGitHubOnlyThePrimaryEmailCounts(t *testing.T) {
	f := newGitHubFake(t)
	f.emails = []map[string]any{
		{"email": "secundario@example.com", "primary": false, "verified": true},
		{"email": "pessoa@example.com", "primary": true, "verified": false},
	}
	out, err := f.g.Exchange(context.Background(), testCode, testVerifier, hashNonce(nonceA), false)
	if err != nil {
		t.Fatal(err)
	}
	id, err := f.g.Verify(context.Background(), out.Ticket, nonceA)
	if err != nil {
		t.Fatal(err)
	}
	if id.Email != "pessoa@example.com" || id.EmailVerified {
		t.Fatalf("um secundário verificado não pode valer pelo principal: %+v", id)
	}
}

func TestGitHubExchangeRefuses(t *testing.T) {
	cases := map[string]struct {
		code, verifier, nonceHash string
		setup                     func(*githubFake)
	}{
		"código errado":         {code: "outro-codigo", verifier: testVerifier, nonceHash: hashNonce(nonceA)},
		"verifier errado":       {code: testCode, verifier: strings.Repeat("x", 43), nonceHash: hashNonce(nonceA)},
		"verifier curto":        {code: testCode, verifier: "curto", nonceHash: hashNonce(nonceA)},
		"código vazio":          {code: "", verifier: testVerifier, nonceHash: hashNonce(nonceA)},
		"código enorme":         {code: strings.Repeat("c", 300), verifier: testVerifier, nonceHash: hashNonce(nonceA)},
		"nonce cru no lugar":    {code: testCode, verifier: testVerifier, nonceHash: nonceA},
		"hash em maiúscula":     {code: testCode, verifier: testVerifier, nonceHash: strings.ToUpper(hashNonce(nonceA))},
		"GitHub recusa a troca": {code: testCode, verifier: testVerifier, nonceHash: hashNonce(nonceA), setup: func(f *githubFake) { f.tokenErr = "incorrect_client_credentials" }},
		"usuário não lido":      {code: testCode, verifier: testVerifier, nonceHash: hashNonce(nonceA), setup: func(f *githubFake) { f.failUser = true }},
		"usuário sem id":        {code: testCode, verifier: testVerifier, nonceHash: hashNonce(nonceA), setup: func(f *githubFake) { f.userID = 0 }},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f := newGitHubFake(t)
			if c.setup != nil {
				c.setup(f)
			}
			if _, err := f.g.Exchange(context.Background(), c.code, c.verifier, c.nonceHash, false); err == nil {
				t.Fatal("a troca tinha de falhar")
			}
		})
	}
}

func TestGitHubFailedExchangeDeletesTheToken(t *testing.T) {
	f := newGitHubFake(t)
	f.failUser = true
	if _, err := f.g.Exchange(context.Background(), testCode, testVerifier, hashNonce(nonceA), true); err == nil {
		t.Fatal("a troca tinha de falhar")
	}
	if len(f.deleted) != 1 {
		t.Fatalf("token de login que não aconteceu não pode ficar vivo, apagados: %v", f.deleted)
	}
}

func TestGitHubVerifyRefuses(t *testing.T) {
	f := newGitHubFake(t)
	now := time.Now()
	good := func(mut func(jwt.MapClaims)) string {
		c := jwt.MapClaims{
			"iss": githubTicketIssuer, "aud": githubTicketAudience, "sub": "583231",
			"email": "pessoa@example.com", "email_verified": true, "nonce": hashNonce(nonceA),
			"iat": now.Unix(), "exp": now.Add(githubTicketTTL).Unix(),
		}
		if mut != nil {
			mut(c)
		}
		s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(f.g.ticketKey)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	// O token de sessão é HS256 com a chave da sessão, não a derivada: não pode passar.
	sessionLike, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss": githubTicketIssuer, "aud": githubTicketAudience, "sub": "583231", "nonce": hashNonce(nonceA),
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	}).SignedString(testSessionKey)
	none, _ := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"iss": githubTicketIssuer, "aud": githubTicketAudience, "sub": "583231", "nonce": hashNonce(nonceA),
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)

	cases := map[string]struct{ ticket, nonce string }{
		"assinado com a chave da sessão": {sessionLike, nonceA},
		"alg none":                       {none, nonceA},
		"vencido":                        {good(func(c jwt.MapClaims) { c["exp"] = now.Add(-2 * time.Minute).Unix() }), nonceA},
		"sem exp":                        {good(func(c jwt.MapClaims) { delete(c, "exp") }), nonceA},
		"emitido no futuro":              {good(func(c jwt.MapClaims) { c["iat"] = now.Add(5 * time.Minute).Unix() }), nonceA},
		"audiência de sessão":            {good(func(c jwt.MapClaims) { c["aud"] = "logn" }), nonceA},
		"outro emissor":                  {good(func(c jwt.MapClaims) { c["iss"] = "https://github.com" }), nonceA},
		"sem sub":                        {good(func(c jwt.MapClaims) { delete(c, "sub") }), nonceA},
		"nonce de outro login":           {good(nil), strings.Repeat("n", 43)},
		"nonce curto":                    {good(nil), "curto"},
		"bilhete vazio":                  {"", nonceA},
		"bilhete enorme":                 {strings.Repeat("a", maxIDTokenLen+1), nonceA},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := f.g.Verify(context.Background(), c.ticket, c.nonce); err == nil {
				t.Fatal("o bilhete tinha de ser recusado")
			}
		})
	}
}

func TestGitHubTicketFromAnotherServerKeyIsRefused(t *testing.T) {
	f := newGitHubFake(t)
	out, err := f.g.Exchange(context.Background(), testCode, testVerifier, hashNonce(nonceA), false)
	if err != nil {
		t.Fatal(err)
	}
	other, err := NewGitHub(testGitHubClient, testGitHubSecret, "logn://oauth/github", []byte("outra-chave"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Verify(context.Background(), out.Ticket, nonceA); err == nil {
		t.Fatal("bilhete de outra chave de sessão não pode valer")
	}
}

// O bilhete não serve de sessão: a chave dele é a derivada, não a da sessão. Mesmo com
// um `user_id` enfiado nas claims e assinado pela chave do bilhete, o servidor não abre
// sessão com ele.
func TestGitHubTicketIsNotASessionToken(t *testing.T) {
	f := newGitHubFake(t)
	previous := domain.JwtSecretKey
	domain.JwtSecretKey = testSessionKey
	t.Cleanup(func() { domain.JwtSecretKey = previous })

	out, err := f.g.Exchange(context.Background(), testCode, testVerifier, hashNonce(nonceA), false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := domain.UserIDFromAccessToken(out.Ticket); err == nil {
		t.Fatal("bilhete do GitHub aceito como token de sessão")
	}
	forged, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": "00000000-0000-0000-0000-000000000001", "exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString(f.g.ticketKey)
	if _, err := domain.UserIDFromAccessToken(forged); err == nil {
		t.Fatal("token assinado com a chave do bilhete aceito como sessão")
	}
}

func TestGitHubDiscardDeletesOnlyTheToken(t *testing.T) {
	f := newGitHubFake(t)
	if err := f.g.Discard(context.Background(), testGitHubToken); err != nil {
		t.Fatal(err)
	}
	if len(f.deleted) != 1 || len(f.revoked) != 0 {
		t.Fatalf("descartar apaga o token e não a autorização: apagados=%v revogados=%v", f.deleted, f.revoked)
	}
}

func TestGitHubDeleteExchangeKeepsTheTokenAndRevokes(t *testing.T) {
	f := newGitHubFake(t)
	out, err := f.g.Exchange(context.Background(), testCode, testVerifier, hashNonce(nonceA), true)
	if err != nil {
		t.Fatal(err)
	}
	if out.AccessToken != testGitHubToken || len(f.deleted) != 0 {
		t.Fatalf("na exclusão o token fica vivo e volta: token=%q apagados=%v", out.AccessToken, f.deleted)
	}
	if err := f.g.Revoke(context.Background(), out.AccessToken, "583231"); err != nil {
		t.Fatal(err)
	}
	if len(f.revoked) != 1 || f.revoked[0] != testGitHubToken {
		t.Fatalf("a autorização tinha de ser revogada: %v", f.revoked)
	}
}

func TestGitHubRevokeRefuses(t *testing.T) {
	cases := map[string]struct {
		token, subject string
		setup          func(*githubFake)
	}{
		"token de outro usuário": {testGitHubToken, "999", nil},
		"token de outro app":     {testGitHubToken, "583231", func(f *githubFake) { f.tokenApp = "Iv1.outroapp" }},
		"sem token":              {"", "583231", nil},
		"sem sub":                {testGitHubToken, "", nil},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f := newGitHubFake(t)
			if c.setup != nil {
				c.setup(f)
			}
			if err := f.g.Revoke(context.Background(), c.token, c.subject); err == nil {
				t.Fatal("a revogação tinha de ser recusada")
			}
			if len(f.revoked) != 0 {
				t.Fatalf("nada podia ser revogado: %v", f.revoked)
			}
		})
	}
}

func TestGitHubHalfConfigIsRefused(t *testing.T) {
	cases := map[string]struct {
		id, secret, redirect string
		key                  []byte
	}{
		"sem client ID": {"", testGitHubSecret, "logn://oauth/github", testSessionKey},
		"sem secret":    {testGitHubClient, "", "logn://oauth/github", testSessionKey},
		"sem redirect":  {testGitHubClient, testGitHubSecret, "", testSessionKey},
		"sem chave":     {testGitHubClient, testGitHubSecret, "logn://oauth/github", nil},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewGitHub(c.id, c.secret, c.redirect, c.key, nil); err == nil {
				t.Fatal("configuração pela metade tinha de ser recusada")
			}
		})
	}
}
