package socialauth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testBundle = "sh.logn.test"

type appleFixture struct {
	key     *rsa.PrivateKey
	server  *httptest.Server
	fetches atomic.Int32
	v       *AppleVerifier
}

func newAppleFixture(t *testing.T) *appleFixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &appleFixture{key: key}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.fetches.Add(1)
		json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "alg": "RS256", "kid": "apple-k1", "use": "sig",
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	}))
	t.Cleanup(f.server.Close)
	f.v = &AppleVerifier{bundleID: testBundle, keys: newJWKSCache(f.server.URL, f.server.Client())}
	return f
}

func (f *appleFixture) sign(t *testing.T, method jwt.SigningMethod, key any, kid string, mut func(jwt.MapClaims)) string {
	t.Helper()
	now := time.Now()
	c := jwt.MapClaims{
		"iss": appleIssuer, "aud": testBundle, "sub": "apple-sub-1",
		"iat": now.Unix(), "exp": now.Add(10 * time.Minute).Unix(),
		"nonce": hashNonce(nonceA), "email": "pessoa@example.com", "email_verified": "true",
	}
	if mut != nil {
		mut(c)
	}
	tok := jwt.NewWithClaims(method, c)
	tok.Header["kid"] = kid
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestAppleAcceptsAGoodToken(t *testing.T) {
	f := newAppleFixture(t)
	id, err := f.v.Verify(context.Background(), f.sign(t, jwt.SigningMethodRS256, f.key, "apple-k1", nil), nonceA)
	if err != nil {
		t.Fatalf("token bom recusado: %v", err)
	}
	if id.Subject != "apple-sub-1" || id.Email != "pessoa@example.com" || !id.EmailVerified || id.IssuedAt.IsZero() {
		t.Fatalf("identidade errada: %+v", id)
	}
}

func TestAppleRefuses(t *testing.T) {
	f := newAppleFixture(t)
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	ec, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	good := func(mut func(jwt.MapClaims)) string {
		return f.sign(t, jwt.SigningMethodRS256, f.key, "apple-k1", mut)
	}

	cases := map[string]struct {
		token string
		nonce string
	}{
		"audiência de outro app":    {good(func(c jwt.MapClaims) { c["aud"] = "com.outro.app" }), nonceA},
		"emissor de outro serviço":  {good(func(c jwt.MapClaims) { c["iss"] = "https://accounts.google.com" }), nonceA},
		"vencido":                   {good(func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-5 * time.Minute).Unix() }), nonceA},
		"sem exp":                   {good(func(c jwt.MapClaims) { delete(c, "exp") }), nonceA},
		"emitido no futuro":         {good(func(c jwt.MapClaims) { c["iat"] = time.Now().Add(10 * time.Minute).Unix() }), nonceA},
		"sem sub":                   {good(func(c jwt.MapClaims) { delete(c, "sub") }), nonceA},
		"nonce de outro login":      {good(nil), nonceB},
		"nonce cru dentro do token": {good(func(c jwt.MapClaims) { c["nonce"] = nonceA }), nonceA},
		"nonce curto":               {good(nil), "x"},
		"assinado por outra chave":  {f.sign(t, jwt.SigningMethodRS256, other, "apple-k1", nil), nonceA},
		"kid desconhecido":          {f.sign(t, jwt.SigningMethodRS256, f.key, "apple-k9", nil), nonceA},
		"algoritmo ES256":           {f.sign(t, jwt.SigningMethodES256, ec, "apple-k1", nil), nonceA},
		"algoritmo none":            {f.sign(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, "apple-k1", nil), nonceA},
		// Confusão de algoritmo: HMAC com a chave pública da Apple como segredo.
		"HS256 com a chave pública": {f.sign(t, jwt.SigningMethodHS256, x509.MarshalPKCS1PublicKey(&f.key.PublicKey), "apple-k1", nil), nonceA},
		"aud em lista sem o bundle": {good(func(c jwt.MapClaims) { c["aud"] = []string{"com.outro.app", "com.mais.um"} }), nonceA},
		"sem iat":                   {good(func(c jwt.MapClaims) { delete(c, "iat") }), nonceA},
		"lixo":                      {"lixo", nonceA},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := f.v.Verify(context.Background(), c.token, c.nonce); err == nil {
				t.Fatal("token aceito")
			}
		})
	}
}

func TestAppleEmailVerifiedNeedsToBeTrue(t *testing.T) {
	f := newAppleFixture(t)
	for _, v := range []any{"false", false, nil} {
		tok := f.sign(t, jwt.SigningMethodRS256, f.key, "apple-k1", func(c jwt.MapClaims) { c["email_verified"] = v })
		id, err := f.v.Verify(context.Background(), tok, nonceA)
		if err != nil {
			t.Fatal(err)
		}
		if id.EmailVerified {
			t.Fatalf("email_verified=%#v virou verdadeiro", v)
		}
	}
}

// Token forjado com `kid` inventado não vira uma busca de chaves por pedido.
func TestAppleUnknownKidDoesNotHammerTheKeyEndpoint(t *testing.T) {
	f := newAppleFixture(t)
	bad := f.sign(t, jwt.SigningMethodRS256, f.key, "inventado", nil)
	for i := 0; i < 20; i++ {
		f.v.Verify(context.Background(), bad, nonceA)
	}
	if n := f.fetches.Load(); n != 1 {
		t.Fatalf("%d buscas de chave para 20 tokens com kid inventado, want 1", n)
	}
}

func TestAppleEmptyBundleIsRefused(t *testing.T) {
	if _, err := NewAppleVerifier(" ", nil); err == nil {
		t.Fatal("bundle vazio aceito")
	}
}

func p8(t *testing.T) (*ecdsa.PrivateKey, string) {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return key, string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func TestAppleRevokeExchangesTheCodeAndRevokes(t *testing.T) {
	key, pemKey := p8(t)
	var revoked atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		// O client_secret é um JWT ES256 da `.p8`, do time e do bundle certos.
		secret, err := jwt.Parse(r.Form.Get("client_secret"), func(*jwt.Token) (any, error) { return &key.PublicKey, nil },
			jwt.WithValidMethods([]string{"ES256"}), jwt.WithIssuer("TEAM1"), jwt.WithAudience(appleIssuer), jwt.WithSubject(testBundle))
		if err != nil || secret.Header["kid"] != "KEY1" || r.Form.Get("client_id") != testBundle {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		switch r.URL.Path {
		case "/token":
			if r.Form.Get("code") != "code-1" || r.Form.Get("grant_type") != "authorization_code" {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error":"invalid_grant"}`))
				return
			}
			// O id_token da troca diz de quem é o código.
			payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"apple-sub-1"}`))
			w.Write([]byte(`{"refresh_token":"rt-1","access_token":"at-1","id_token":"h.` + payload + `.s"}`))
		case "/revoke":
			revoked.Store(r.Form.Get("token") + "/" + r.Form.Get("token_type_hint"))
		}
	}))
	defer srv.Close()

	rv, err := NewAppleRevoker("TEAM1", "KEY1", testBundle, pemKey, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	rv.tokenURL, rv.revokeURL = srv.URL+"/token", srv.URL+"/revoke"

	// Código de outra pessoa: a troca responde com outro `sub`, e nada é revogado.
	if err := rv.Revoke(context.Background(), "code-1", "apple-sub-de-outra-pessoa"); err == nil {
		t.Fatal("código de outro sub revogado")
	}
	if revoked.Load() != nil {
		t.Fatal("revogou o token de outra pessoa")
	}

	if err := rv.Revoke(context.Background(), "code-1", "apple-sub-1"); err != nil {
		t.Fatalf("revogação: %v", err)
	}
	if got, _ := revoked.Load().(string); got != "rt-1/refresh_token" {
		t.Fatalf("revogou %q, want o refresh token", got)
	}

	err = rv.Revoke(context.Background(), "code-velho", "apple-sub-1")
	if err == nil || !strings.Contains(err.Error(), "invalid_grant") {
		t.Fatalf("código recusado pela Apple virou %v", err)
	}
	// O erro vai para o log: o client_secret não pode estar nele.
	if strings.Contains(err.Error(), "eyJ") {
		t.Fatalf("o erro leva um JWT: %v", err)
	}
	if err := rv.Revoke(context.Background(), "", "apple-sub-1"); err == nil {
		t.Fatal("revogação sem código aceita")
	}
}

// Com a Apple fora do ar, a chave vencida vale por um tempo, e depois não.
func TestAppleStaleKeysHaveACeiling(t *testing.T) {
	f := newAppleFixture(t)
	tok := f.sign(t, jwt.SigningMethodRS256, f.key, "apple-k1", nil)
	if _, err := f.v.Verify(context.Background(), tok, nonceA); err != nil {
		t.Fatal(err)
	}
	f.server.Close() // a Apple sai do ar

	c := f.v.keys
	c.mu.Lock()
	c.fetchedAt = time.Now().Add(-2 * jwksTTL)
	c.triedAt = time.Time{}
	c.mu.Unlock()
	if _, err := f.v.Verify(context.Background(), tok, nonceA); err != nil {
		t.Fatalf("chave vencida há pouco, com a Apple fora, recusada: %v", err)
	}

	c.mu.Lock()
	c.fetchedAt = time.Now().Add(-jwksMaxStale - time.Hour)
	c.triedAt = time.Time{}
	c.mu.Unlock()
	if _, err := f.v.Verify(context.Background(), tok, nonceA); err == nil {
		t.Fatal("chave vencida há mais de um dia aceita")
	}
}

// Muitos logins ao mesmo tempo com o cache vazio fazem uma busca só.
func TestAppleConcurrentLoginsShareOneFetch(t *testing.T) {
	f := newAppleFixture(t)
	tok := f.sign(t, jwt.SigningMethodRS256, f.key, "apple-k1", nil)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := f.v.Verify(context.Background(), tok, nonceA); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if n := f.fetches.Load(); n != 1 {
		t.Fatalf("%d buscas para 20 logins simultâneos, want 1", n)
	}
}

func TestAppleRevokerRefusesABadKey(t *testing.T) {
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	der, _ := x509.MarshalPKCS8PrivateKey(rsaKey)
	rsaPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	_, good := p8(t)
	for name, c := range map[string][4]string{
		"não é PEM":  {"T", "K", testBundle, "lixo"},
		"chave RSA":  {"T", "K", testBundle, rsaPEM},
		"sem time":   {"", "K", testBundle, good},
		"sem key id": {"T", "", testBundle, good},
		"sem bundle": {"T", "K", "", good},
	} {
		if _, err := NewAppleRevoker(c[0], c[1], c[2], c[3], nil); err == nil {
			t.Errorf("%s: aceito", name)
		}
	}
}
