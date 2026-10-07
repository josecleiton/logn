package socialauth

import (
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ProviderApple é o nome do provedor na API e em `user_identities.provider`.
const ProviderApple = "apple"

const (
	appleIssuer    = "https://appleid.apple.com"
	appleKeysURL   = "https://appleid.apple.com/auth/keys"
	appleTokenURL  = "https://appleid.apple.com/auth/token"
	appleRevokeURL = "https://appleid.apple.com/auth/revoke"
)

// AppleVerifier aceita só tokens emitidos pela Apple para o bundle do app (ADR 0017).
type AppleVerifier struct {
	bundleID string
	keys     *jwksCache
}

// NewAppleVerifier recusa bundle vazio: sem ele, a audiência não seria conferida.
func NewAppleVerifier(bundleID string, client *http.Client) (*AppleVerifier, error) {
	if strings.TrimSpace(bundleID) == "" {
		return nil, errors.New("socialauth: empty bundle")
	}
	if client == nil {
		client = &http.Client{Timeout: fetchTimeout}
	}
	return &AppleVerifier{bundleID: bundleID, keys: newJWKSCache(appleKeysURL, client)}, nil
}

func (a *AppleVerifier) Verify(ctx context.Context, idToken, rawNonce string) (Identity, error) {
	if idToken == "" || len(idToken) > maxIDTokenLen {
		return Identity{}, errors.New("token empty or too large")
	}
	if len(rawNonce) < minNonceLen || len(rawNonce) > maxNonceLen {
		return Identity{}, errors.New("nonce length out of range")
	}

	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	claims := jwt.MapClaims{}
	// RS256 fixo, emissor, audiência e prazo pelo `golang-jwt`; a chave, pelo `kid`,
	// entre as publicadas pela Apple. O `alg` do token nunca escolhe o algoritmo.
	_, err := jwt.ParseWithClaims(idToken, claims, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		return a.keys.key(ctx, kid)
	},
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(appleIssuer),
		jwt.WithAudience(a.bundleID),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(clockSkew),
	)
	if err != nil {
		return Identity{}, err
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		return Identity{}, errors.New("token without sub")
	}
	// Mesmo desenho do Google: o pedido à Apple leva o SHA-256 do nonce, e o token
	// volta com ele.
	tokenNonce, _ := claims["nonce"].(string)
	if subtle.ConstantTimeCompare([]byte(tokenNonce), []byte(hashNonce(rawNonce))) != 1 {
		return Identity{}, errors.New("nonce mismatch")
	}

	issuedAt, err := claims.GetIssuedAt()
	if err != nil || issuedAt == nil {
		return Identity{}, errors.New("token without iat")
	}
	email, _ := claims["email"].(string)
	return Identity{
		Subject: sub,
		Email:   email,
		// A Apple manda "true" como string; o relay do "Ocultar meu e-mail" também
		// vem verificado, e só não bate com conta nenhuma.
		EmailVerified: claimTrue(claims["email_verified"]),
		IssuedAt:      issuedAt.Time,
	}, nil
}

// jwksCache guarda as chaves públicas de um provedor. Busca de novo quando vencem ou
// quando chega um `kid` desconhecido, que é como a rotação de chave aparece; a busca
// por `kid` desconhecido tem intervalo mínimo, para token forjado não virar enxurrada
// de pedidos ao provedor. A busca roda fora do lock e uma por vez: com a Apple lenta,
// quem só precisa de uma chave já em cache não espera na fila.
type jwksCache struct {
	url    string
	client *http.Client

	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
	// A última tentativa, com ou sem sucesso: o intervalo mínimo vale também quando a
	// Apple está fora, para cada pedido não virar mais uma busca.
	triedAt time.Time
	// Fechado quando a busca em andamento termina. Nil sem busca no ar.
	inflight chan struct{}
}

const (
	jwksTTL        = time.Hour
	jwksMinRefetch = time.Minute
	// Com a Apple fora do ar, a chave vencida segue valendo até aqui, e depois não:
	// uma chave que a Apple tirou do ar não pode valer para sempre.
	jwksMaxStale     = 24 * time.Hour
	maxJWKSBodyBytes = 64 << 10
)

func newJWKSCache(url string, client *http.Client) *jwksCache {
	return &jwksCache{url: url, client: client}
}

func (c *jwksCache) lookup(kid string) (key *rsa.PublicKey, known bool, age, sinceTry time.Duration, fetching bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key, known = c.keys[kid]
	return key, known, time.Since(c.fetchedAt), time.Since(c.triedAt), c.inflight != nil
}

func (c *jwksCache) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	k, known, age, sinceTry, fetching := c.lookup(kid)
	if known && age < jwksTTL {
		return k, nil
	}
	// Chave conhecida e vencida, ou `kid` novo: busca, respeitando o intervalo mínimo
	// entre tentativas. Com uma busca no ar, espera por ela: quem chega junto do
	// primeiro login não pode falhar só porque a tentativa dele é recente.
	if fetching || sinceTry >= jwksMinRefetch {
		c.refresh(ctx)
	}
	k, known, age, _, _ = c.lookup(kid)
	if known && age < jwksMaxStale {
		return k, nil
	}
	if known {
		return nil, fmt.Errorf("key stale for too long: %q", clip(kid))
	}
	return nil, fmt.Errorf("unknown kid: %q", clip(kid))
}

// refresh busca as chaves, ou espera a busca que já está no ar.
func (c *jwksCache) refresh(ctx context.Context) {
	c.mu.Lock()
	if ch := c.inflight; ch != nil {
		c.mu.Unlock()
		select {
		case <-ch:
		case <-ctx.Done():
		}
		return
	}
	ch := make(chan struct{})
	c.inflight = ch
	c.triedAt = time.Now()
	c.mu.Unlock()

	keys, err := c.fetch(ctx)

	c.mu.Lock()
	if err == nil {
		c.keys = keys
		c.fetchedAt = time.Now()
	}
	c.inflight = nil
	c.mu.Unlock()
	close(ch)
}

func (c *jwksCache) fetch(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("keys: status %d", resp.StatusCode)
	}
	var doc struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			Alg string `json:"alg"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxJWKSBodyBytes)).Decode(&doc); err != nil {
		return nil, err
	}
	keys := map[string]*rsa.PublicKey{}
	for _, k := range doc.Keys {
		if k.Kty != "RSA" || (k.Alg != "" && k.Alg != "RS256") {
			continue
		}
		n, errN := base64.RawURLEncoding.DecodeString(k.N)
		e, errE := base64.RawURLEncoding.DecodeString(k.E)
		if errN != nil || errE != nil || len(e) == 0 || len(e) > 4 {
			continue
		}
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	}
	if len(keys) == 0 {
		return nil, errors.New("keys: no valid RSA key")
	}
	return keys, nil
}

// Revoker desfaz, no provedor, a autorização que o jogador deu ao app. A Apple exige
// isso na exclusão da conta.
//
// `subject` é o `sub` que provou a posse da conta: o código tem de ser dele, ou nada é
// revogado. Sem isso, um código de outra pessoa revogaria o acesso dela.
type Revoker interface {
	Revoke(ctx context.Context, authorizationCode, subject string) error
}

// AppleRevoker troca o `authorization_code` de um login recente pelo refresh token e
// o revoga na hora. Assim nenhum token da Apple fica guardado no banco.
type AppleRevoker struct {
	teamID, keyID, bundleID string
	key                     *ecdsa.PrivateKey
	client                  *http.Client
	tokenURL, revokeURL     string
}

// NewAppleRevoker lê a chave `.p8` (PKCS#8, PEM) do Sign in with Apple.
func NewAppleRevoker(teamID, keyID, bundleID, privateKeyPEM string, client *http.Client) (*AppleRevoker, error) {
	if teamID == "" || keyID == "" || bundleID == "" {
		return nil, errors.New("socialauth: empty team, key or bundle")
	}
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, errors.New("socialauth: Apple key is not PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("socialauth: Apple key: %w", err)
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("socialauth: Apple key is not ECDSA")
	}
	if client == nil {
		client = &http.Client{Timeout: fetchTimeout}
	}
	return &AppleRevoker{
		teamID: teamID, keyID: keyID, bundleID: bundleID, key: key, client: client,
		tokenURL: appleTokenURL, revokeURL: appleRevokeURL,
	}, nil
}

// clientSecret é o JWT que a Apple pede no lugar de um secret fixo: ES256, assinado
// com a `.p8`, curto de propósito.
func (a *AppleRevoker) clientSecret() (string, error) {
	now := time.Now()
	t := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.RegisteredClaims{
		Issuer:    a.teamID,
		Subject:   a.bundleID,
		Audience:  jwt.ClaimStrings{appleIssuer},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
	})
	t.Header["kid"] = a.keyID
	return t.SignedString(a.key)
}

func (a *AppleRevoker) Revoke(ctx context.Context, authorizationCode, subject string) error {
	if authorizationCode == "" || subject == "" {
		return errors.New("missing authorization_code or sub")
	}
	secret, err := a.clientSecret()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*fetchTimeout)
	defer cancel()

	var tokens struct {
		RefreshToken string `json:"refresh_token"`
		AccessToken  string `json:"access_token"`
		IDToken      string `json:"id_token"`
	}
	if err := a.post(ctx, a.tokenURL, url.Values{
		"client_id":     {a.bundleID},
		"client_secret": {secret},
		"code":          {authorizationCode},
		"grant_type":    {"authorization_code"},
	}, &tokens); err != nil {
		return fmt.Errorf("code exchange: %w", err)
	}

	// O `id_token` desta resposta veio direto da Apple, por TLS, ao pedido do próprio
	// servidor; dele só se lê de quem é o código.
	if got := unverifiedSubject(tokens.IDToken); subtle.ConstantTimeCompare([]byte(got), []byte(subject)) != 1 {
		return errors.New("code belongs to another sub; nothing revoked")
	}

	token, hint := tokens.RefreshToken, "refresh_token"
	if token == "" {
		token, hint = tokens.AccessToken, "access_token"
	}
	if token == "" {
		return errors.New("code exchange returned no token")
	}
	if err := a.post(ctx, a.revokeURL, url.Values{
		"client_id":       {a.bundleID},
		"client_secret":   {secret},
		"token":           {token},
		"token_type_hint": {hint},
	}, nil); err != nil {
		return fmt.Errorf("revocation: %w", err)
	}
	return nil
}

func unverifiedSubject(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ""
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Sub string `json:"sub"`
	}
	if json.Unmarshal(raw, &claims) != nil {
		return ""
	}
	return claims.Sub
}

func (a *AppleRevoker) post(ctx context.Context, endpoint string, form url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// O corpo de erro da Apple traz só um código (`invalid_grant`...), sem dado
		// do jogador; vai para o log cortado.
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("status %d: %s", resp.StatusCode, clip(string(body)))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(io.LimitReader(resp.Body, maxJWKSBodyBytes)).Decode(out)
}
