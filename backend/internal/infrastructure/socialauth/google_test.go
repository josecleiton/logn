package socialauth

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"

	"google.golang.org/api/idtoken"
	"google.golang.org/api/option"
)

const testAudience = "test-client.apps.googleusercontent.com"

// Nonces do tamanho que o app manda.
var (
	nonceA = strings.Repeat("a", 43)
	nonceB = strings.Repeat("b", 43)
)

// fakeToken monta só o que o verificador lê antes de delegar: o cabeçalho. Assinatura
// e payload são da validação trocada.
func fakeToken(alg string) string {
	head := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"` + alg + `","kid":"k1"}`))
	return head + ".e30.c2ln"
}

func verifierReturning(p *idtoken.Payload, err error) *GoogleVerifier {
	return &GoogleVerifier{
		clients: []GoogleClient{{Audience: testAudience, AuthorizedParty: testAudience}},
		validate: func(_ context.Context, _, aud string) (*idtoken.Payload, error) {
			if aud != testAudience {
				return nil, errors.New("audiência errada chegou à validação")
			}
			return p, err
		},
	}
}

func goodPayload(nonce string) *idtoken.Payload {
	return &idtoken.Payload{
		Issuer:   "https://accounts.google.com",
		Subject:  "sub-1",
		IssuedAt: time.Now().Unix(),
		Claims: map[string]any{
			"nonce":          hashNonce(nonce),
			"email":          "pessoa@example.com",
			"email_verified": true,
			"azp":            testAudience,
		},
	}
}

func TestVerifyAcceptsAGoodToken(t *testing.T) {
	v := verifierReturning(goodPayload(nonceA), nil)
	id, err := v.Verify(context.Background(), fakeToken("RS256"), nonceA)
	if err != nil {
		t.Fatalf("token bom recusado: %v", err)
	}
	if id.Subject != "sub-1" || id.Email != "pessoa@example.com" || !id.EmailVerified || id.IssuedAt.IsZero() {
		t.Fatalf("identidade errada: %+v", id)
	}
}

func TestVerifyRefuses(t *testing.T) {
	wrongIssuer := goodPayload(nonceA)
	wrongIssuer.Issuer = "https://cloud.google.com/iap"
	noSub := goodPayload(nonceA)
	noSub.Subject = ""
	noNonce := goodPayload(nonceA)
	delete(noNonce.Claims, "nonce")
	// O token carrega o hash; o nonce cru dentro dele não pode passar.
	rawInToken := goodPayload(nonceA)
	rawInToken.Claims["nonce"] = nonceA
	otherClient := goodPayload(nonceA)
	otherClient.Claims["azp"] = "outro-client.apps.googleusercontent.com"
	future := goodPayload(nonceA)
	future.IssuedAt = time.Now().Add(10 * time.Minute).Unix()

	cases := []struct {
		name    string
		payload *idtoken.Payload
		err     error
		token   string
		nonce   string
	}{
		{"validação falhou (assinatura, aud ou exp)", nil, errors.New("idtoken: token expired"), fakeToken("RS256"), nonceA},
		{"algoritmo ES256", goodPayload(nonceA), nil, fakeToken("ES256"), nonceA},
		{"algoritmo none", goodPayload(nonceA), nil, fakeToken("none"), nonceA},
		{"sem cabeçalho", goodPayload(nonceA), nil, "lixo", nonceA},
		{"emissor de outro serviço", wrongIssuer, nil, fakeToken("RS256"), nonceA},
		{"sem sub", noSub, nil, fakeToken("RS256"), nonceA},
		{"pedido por outro client", otherClient, nil, fakeToken("RS256"), nonceA},
		{"emitido no futuro", future, nil, fakeToken("RS256"), nonceA},
		{"nonce de outro login", goodPayload(nonceA), nil, fakeToken("RS256"), nonceB},
		{"token sem nonce", noNonce, nil, fakeToken("RS256"), nonceA},
		{"hash mandado como nonce", goodPayload(nonceA), nil, fakeToken("RS256"), hashNonce(nonceA)},
		{"nonce cru dentro do token", rawInToken, nil, fakeToken("RS256"), nonceA},
		{"nonce vazio", goodPayload(""), nil, fakeToken("RS256"), ""},
		{"nonce curto", goodPayload("x"), nil, fakeToken("RS256"), "x"},
		{"token grande demais", goodPayload(nonceA), nil, fakeToken("RS256") + strings.Repeat("a", maxIDTokenLen), nonceA},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := verifierReturning(c.payload, c.err)
			if _, err := v.Verify(context.Background(), c.token, c.nonce); err == nil {
				t.Fatal("token aceito")
			}
		})
	}
}

func TestEmailVerifiedNeedsToBeTrue(t *testing.T) {
	for _, v := range []any{false, "false", nil, "yes", 1} {
		p := goodPayload(nonceA)
		p.Claims["email_verified"] = v
		id, err := verifierReturning(p, nil).Verify(context.Background(), fakeToken("RS256"), nonceA)
		if err != nil {
			t.Fatalf("%v: %v", v, err)
		}
		if id.EmailVerified {
			t.Fatalf("email_verified=%#v virou verdadeiro", v)
		}
	}
	p := goodPayload(nonceA)
	p.Claims["email_verified"] = "true"
	id, _ := verifierReturning(p, nil).Verify(context.Background(), fakeToken("RS256"), nonceA)
	if !id.EmailVerified {
		t.Fatal(`email_verified="true" recusado`)
	}
}

func TestEmptyAudienceIsRefused(t *testing.T) {
	for _, aud := range []string{"", "  "} {
		if _, err := NewGoogleVerifier(GoogleClient{Audience: aud, AuthorizedParty: testAudience}); err == nil {
			t.Fatalf("audiência %q aceita", aud)
		}
		if _, err := NewGoogleVerifier(GoogleClient{Audience: testAudience, AuthorizedParty: aud}); err == nil {
			t.Fatalf("azp %q aceito", aud)
		}
	}
	if _, err := NewGoogleVerifier(); err == nil {
		t.Fatal("verificador sem client aceito")
	}
}

const (
	testWebClient     = "web-client.apps.googleusercontent.com"
	testAndroidClient = "android-client.apps.googleusercontent.com"
	// Outra chave que assina o app (debug, upload, Play App Signing): outro client.
	testAndroidDebug = "android-debug.apps.googleusercontent.com"
)

// O Android pede o token em nome do client web: `aud` web, `azp` Android (ADR 0022).
// O par é o que vale, nunca uma das pontas sozinha.
func TestAndroidPair(t *testing.T) {
	v := &GoogleVerifier{
		clients: []GoogleClient{
			{Audience: testAudience, AuthorizedParty: testAudience},
			{Audience: testWebClient, AuthorizedParty: testAndroidClient},
			{Audience: testWebClient, AuthorizedParty: testAndroidDebug},
		},
	}
	token := func(aud, azp string) *GoogleVerifier {
		v.validate = func(_ context.Context, _, want string) (*idtoken.Payload, error) {
			if want != aud {
				return nil, errors.New("idtoken: audience provided does not match aud claim in the JWT")
			}
			p := goodPayload(nonceA)
			if azp == "" {
				delete(p.Claims, "azp")
			} else {
				p.Claims["azp"] = azp
			}
			return p, nil
		}
		return v
	}

	if _, err := token(testWebClient, testAndroidClient).Verify(context.Background(), fakeToken("RS256"), nonceA); err != nil {
		t.Fatalf("token do Android recusado: %v", err)
	}
	// O segundo client Android da mesma audiência web também vale: o par não é o
	// primeiro que a audiência achou.
	if _, err := token(testWebClient, testAndroidDebug).Verify(context.Background(), fakeToken("RS256"), nonceA); err != nil {
		t.Fatalf("token do segundo client Android recusado: %v", err)
	}
	if _, err := token(testAudience, testAudience).Verify(context.Background(), fakeToken("RS256"), nonceA); err != nil {
		t.Fatalf("token do iOS recusado: %v", err)
	}
	if _, err := token(testAudience, "").Verify(context.Background(), fakeToken("RS256"), nonceA); err != nil {
		t.Fatalf("token do iOS sem azp recusado: %v", err)
	}
	for name, c := range map[string][2]string{
		"web pedido por outro client":  {testWebClient, "outro-client.apps.googleusercontent.com"},
		"web pedido pelo iOS":          {testWebClient, testAudience},
		"web pedido pelo próprio web":  {testWebClient, testWebClient},
		"web sem azp":                  {testWebClient, ""},
		"iOS pedido pelo Android":      {testAudience, testAndroidClient},
		"iOS pedido pelo debug":        {testAudience, testAndroidDebug},
		"audiência do Android sozinha": {testAndroidClient, testAndroidClient},
		"audiência de outro app":       {"outro.apps.googleusercontent.com", testAndroidClient},
	} {
		if _, err := token(c[0], c[1]).Verify(context.Background(), fakeToken("RS256"), nonceA); err == nil {
			t.Errorf("%s: token aceito", name)
		}
	}
}

// jwksTransport responde às chaves públicas do Google com as do teste. O resto do
// caminho é o `idtoken` de verdade: assinatura, `aud` e `exp` são conferidos por ele.
type jwksTransport struct{ body []byte }

func (j jwksTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.String() != "https://www.googleapis.com/oauth2/v3/certs" {
		return nil, errors.New("pedido inesperado: " + r.URL.String())
	}
	return &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(j.body)),
		Request:    r,
	}, nil
}

func signRS256(t *testing.T, key *rsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	enc := func(v any) string {
		raw, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(raw)
	}
	signing := enc(map[string]string{"alg": "RS256", "kid": kid, "typ": "JWT"}) + "." + enc(claims)
	sum := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return signing + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func TestVerifyWithTheRealValidator(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	jwks, _ := json.Marshal(map[string]any{"keys": []map[string]string{{
		"kty": "RSA", "alg": "RS256", "use": "sig", "kid": "k1",
		"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
	}}})

	ctx := context.Background()
	validator, err := idtoken.NewValidator(ctx, option.WithHTTPClient(&http.Client{Transport: jwksTransport{jwks}}))
	if err != nil {
		t.Fatal(err)
	}
	v := &GoogleVerifier{
		clients: []GoogleClient{
			{Audience: testAudience, AuthorizedParty: testAudience},
			{Audience: testWebClient, AuthorizedParty: testAndroidClient},
		},
		validate: validator.Validate,
	}

	now := time.Now()
	claims := func(mut func(map[string]any)) map[string]any {
		c := map[string]any{
			"iss": "https://accounts.google.com", "aud": testAudience, "azp": testAudience,
			"sub": "sub-1", "email": "pessoa@example.com", "email_verified": true,
			"iat": now.Unix(), "exp": now.Add(time.Hour).Unix(), "nonce": hashNonce(nonceA),
		}
		if mut != nil {
			mut(c)
		}
		return c
	}

	if _, err := v.Verify(ctx, signRS256(t, key, "k1", claims(nil)), nonceA); err != nil {
		t.Fatalf("token bom recusado: %v", err)
	}
	android := claims(func(c map[string]any) { c["aud"] = testWebClient; c["azp"] = testAndroidClient })
	if _, err := v.Verify(ctx, signRS256(t, key, "k1", android), nonceA); err != nil {
		t.Fatalf("token do Android recusado pela validação de verdade: %v", err)
	}

	refused := map[string]string{
		"audiência de outro app":   signRS256(t, key, "k1", claims(func(c map[string]any) { c["aud"] = "outro.apps.googleusercontent.com" })),
		"web com azp do iOS":       signRS256(t, key, "k1", claims(func(c map[string]any) { c["aud"] = testWebClient })),
		"vencido":                  signRS256(t, key, "k1", claims(func(c map[string]any) { c["exp"] = now.Add(-time.Minute).Unix() })),
		"assinado por outra chave": signRS256(t, other, "k1", claims(nil)),
		"chave desconhecida":       signRS256(t, key, "k2", claims(nil)),
	}
	for name, token := range refused {
		if _, err := v.Verify(ctx, token, nonceA); err == nil {
			t.Errorf("%s: token aceito", name)
		}
	}
	// Payload trocado depois da assinatura.
	good := signRS256(t, key, "k1", claims(nil))
	parts := strings.Split(good, ".")
	forged, _ := json.Marshal(claims(func(c map[string]any) { c["sub"] = "sub-de-outra-pessoa" }))
	parts[1] = base64.RawURLEncoding.EncodeToString(forged)
	if _, err := v.Verify(ctx, strings.Join(parts, "."), nonceA); err == nil {
		t.Error("payload adulterado aceito")
	}
}
