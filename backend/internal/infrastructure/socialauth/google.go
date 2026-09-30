// Package socialauth confere o ID token que um provedor externo emitiu para o app e
// devolve quem ele identifica (ADR 0016).
package socialauth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"google.golang.org/api/idtoken"
)

// ProviderGoogle é o nome do provedor na API e em `user_identities.provider`.
const ProviderGoogle = "google"

// Identity é o que o servidor aproveita de um ID token conferido.
type Identity struct {
	Subject       string
	Email         string
	EmailVerified bool
	// Quando o provedor emitiu o token. A exclusão de conta exige login recente.
	IssuedAt time.Time
}

// Verifier confere o token e o nonce e devolve a identidade. Erro é sempre token
// recusado; o motivo vai para o log, nunca para a resposta.
type Verifier interface {
	Verify(ctx context.Context, idToken, rawNonce string) (Identity, error)
}

// maxIDTokenLen barra lixo antes de qualquer decodificação. Um ID token do Google tem
// perto de 1,2 KB.
const maxIDTokenLen = 8 << 10

// O app manda 32 bytes em base64url, 43 caracteres. Menos que isso é nonce que dá para
// adivinhar.
const (
	minNonceLen = 32
	maxNonceLen = 128
)

// clockSkew é a folga para o relógio do Google estar um pouco à frente do nosso.
const clockSkew = time.Minute

// fetchTimeout segura a busca das chaves públicas do Google quando o cache delas
// venceu. Sem ele, um endpoint travado prendia o pedido até o timeout do servidor.
const fetchTimeout = 5 * time.Second

// maxLoggedLen corta o que vem do token antes de ir para o log.
const maxLoggedLen = 64

// validateFunc é a assinatura de `idtoken.Validate`, trocada nos testes.
type validateFunc func(ctx context.Context, idToken, audience string) (*idtoken.Payload, error)

// GoogleClient é um par aceito de `aud` (para quem o token foi emitido) e `azp` (o
// client que o pediu).
//
// No iOS os dois são o client iOS. No Android (ADR 0022) o Credential Manager pede o
// token em nome do client web (`serverClientId`), e ele volta com `aud` = client web e
// `azp` = client Android: aceitar só o `aud` deixaria passar token pedido por qualquer
// outro client do mesmo projeto.
type GoogleClient struct {
	Audience        string
	AuthorizedParty string
}

// GoogleVerifier aceita só tokens emitidos pelo Google para um dos pares do app.
type GoogleVerifier struct {
	clients  []GoogleClient
	validate validateFunc
}

// NewGoogleVerifier recusa par com campo vazio: `idtoken.Validate` pula a checagem de
// `aud` quando ela vem vazia, e aí qualquer token do Google para qualquer app passaria.
func NewGoogleVerifier(clients ...GoogleClient) (*GoogleVerifier, error) {
	if len(clients) == 0 {
		return nil, errors.New("socialauth: nenhum client do Google")
	}
	for _, c := range clients {
		if strings.TrimSpace(c.Audience) == "" || strings.TrimSpace(c.AuthorizedParty) == "" {
			return nil, errors.New("socialauth: client do Google com audiência ou azp vazio")
		}
	}
	return &GoogleVerifier{clients: clients, validate: idtoken.Validate}, nil
}

var googleIssuers = map[string]bool{
	"accounts.google.com":         true,
	"https://accounts.google.com": true,
}

func (g *GoogleVerifier) Verify(ctx context.Context, idToken, rawNonce string) (Identity, error) {
	if idToken == "" || len(idToken) > maxIDTokenLen {
		return Identity{}, errors.New("token vazio ou grande demais")
	}
	if len(rawNonce) < minNonceLen || len(rawNonce) > maxNonceLen {
		return Identity{}, errors.New("nonce fora do tamanho")
	}

	// O algoritmo é fixo. `idtoken.Validate` também aceita ES256, conferido contra as
	// chaves do IAP; um token assinado por elas não é de login e não tem o que fazer aqui.
	if alg, err := headerAlg(idToken); err != nil || alg != "RS256" {
		return Identity{}, fmt.Errorf("algoritmo recusado: %q", clip(alg))
	}

	// Assinatura contra as chaves públicas do Google, `aud` e `exp`, contra cada
	// audiência aceita. É a validação inteira em cada tentativa: nada do token é lido
	// antes de a assinatura conferir.
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	var payload *idtoken.Payload
	var audience string
	var err error
	tried := map[string]bool{}
	for _, c := range g.clients {
		if tried[c.Audience] {
			continue
		}
		tried[c.Audience] = true
		if payload, err = g.validate(ctx, idToken, c.Audience); err == nil {
			audience = c.Audience
			break
		}
	}
	if err != nil {
		return Identity{}, err
	}
	if !googleIssuers[payload.Issuer] {
		return Identity{}, fmt.Errorf("emissor recusado: %q", clip(payload.Issuer))
	}
	if payload.Subject == "" {
		return Identity{}, errors.New("token sem sub")
	}
	// `azp` é o client que pediu o token, e tem de ser o de um dos pares da audiência que
	// conferiu (no Android, um por chave que assina o app). Ausente, só vale no par em
	// que o client pede para si mesmo (o iOS): no Android o `azp` é o que distingue o
	// nosso client de qualquer outro do projeto.
	azp, _ := payload.Claims["azp"].(string)
	if !g.authorized(audience, azp) {
		return Identity{}, fmt.Errorf("azp recusado: %q", clip(azp))
	}
	issuedAt := time.Unix(payload.IssuedAt, 0)
	if issuedAt.After(time.Now().Add(clockSkew)) {
		return Identity{}, errors.New("token emitido no futuro")
	}

	// O pedido de login leva o SHA-256 do nonce, e o token volta com ele. O app guarda
	// o nonce cru e só o manda para cá: quem tiver copiado o token não tem o que mandar
	// junto, porque o token só carrega o hash.
	tokenNonce, _ := payload.Claims["nonce"].(string)
	want := hashNonce(rawNonce)
	if subtle.ConstantTimeCompare([]byte(tokenNonce), []byte(want)) != 1 {
		return Identity{}, errors.New("nonce não confere")
	}

	email, _ := payload.Claims["email"].(string)
	return Identity{
		Subject:       payload.Subject,
		Email:         email,
		EmailVerified: claimTrue(payload.Claims["email_verified"]),
		IssuedAt:      issuedAt,
	}, nil
}

// authorized diz se algum par da audiência aceita o `azp`.
func (g *GoogleVerifier) authorized(audience, azp string) bool {
	for _, c := range g.clients {
		if c.Audience != audience {
			continue
		}
		if azp == c.AuthorizedParty || (azp == "" && c.AuthorizedParty == c.Audience) {
			return true
		}
	}
	return false
}

func clip(s string) string {
	if len(s) > maxLoggedLen {
		return s[:maxLoggedLen]
	}
	return s
}

// hashNonce é o que o app manda ao Google: SHA-256 do nonce cru, em hex minúsculo.
func hashNonce(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// claimTrue aceita o booleano e a string "true": o Google já mandou `email_verified`
// dos dois jeitos.
func claimTrue(v any) bool {
	switch b := v.(type) {
	case bool:
		return b
	case string:
		return b == "true"
	}
	return false
}

func headerAlg(token string) (string, error) {
	head, _, ok := strings.Cut(token, ".")
	if !ok {
		return "", errors.New("token sem cabeçalho")
	}
	raw, err := base64.RawURLEncoding.DecodeString(head)
	if err != nil {
		return "", err
	}
	var h struct {
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(raw, &h); err != nil {
		return "", err
	}
	return h.Alg, nil
}
