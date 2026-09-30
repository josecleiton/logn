package socialauth

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ProviderGitHub é o nome do provedor na API e em `user_identities.provider`.
const ProviderGitHub = "github"

const (
	githubOAuthURL = "https://github.com"
	githubAPIURL   = "https://api.github.com"

	// O bilhete é assinado e conferido só por este servidor; `iss` e `aud` o separam
	// do token de sessão, além da chave.
	githubTicketIssuer   = "logn"
	githubTicketAudience = "logn-github-ticket"
	githubTicketTTL      = 10 * time.Minute
	// Rótulo da derivação da chave do bilhete a partir de JWT_SECRET. Trocar o rótulo
	// invalida os bilhetes em voo, e só eles.
	githubTicketKeyLabel = "logn/github-ticket/v1"

	// O código do GitHub tem 20 caracteres; o verifier, de 43 a 128 (RFC 7636).
	maxGitHubCodeLen   = 256
	minCodeVerifierLen = 43
	maxCodeVerifierLen = 128
	// Corpo das respostas do GitHub que o servidor lê. `/user` de uma conta cheia fica
	// perto de 2 KB.
	maxGitHubBodyBytes = 64 << 10
)

// GitHub troca o código do login pelo bilhete, confere o bilhete e revoga o acesso na
// exclusão (ADR 0019). É o único que fala com o GitHub, e o único que vê o secret.
type GitHub struct {
	clientID, clientSecret, redirectURI string
	ticketKey                           []byte
	client                              *http.Client
	oauthURL, apiURL                    string
}

// NewGitHub recusa configuração pela metade: sem client ID, secret, redirect ou a chave
// da sessão, não há como trocar o código nem assinar o bilhete.
func NewGitHub(clientID, clientSecret, redirectURI string, sessionKey []byte, client *http.Client) (*GitHub, error) {
	if strings.TrimSpace(clientID) == "" || strings.TrimSpace(clientSecret) == "" {
		return nil, errors.New("socialauth: client ID ou secret do GitHub vazio")
	}
	if strings.TrimSpace(redirectURI) == "" {
		return nil, errors.New("socialauth: redirect do GitHub vazio")
	}
	if len(sessionKey) == 0 {
		return nil, errors.New("socialauth: chave da sessão vazia")
	}
	if client == nil {
		client = &http.Client{Timeout: fetchTimeout}
	}
	mac := hmac.New(sha256.New, sessionKey)
	mac.Write([]byte(githubTicketKeyLabel))
	return &GitHub{
		clientID: clientID, clientSecret: clientSecret, redirectURI: redirectURI,
		ticketKey: mac.Sum(nil), client: client,
		oauthURL: githubOAuthURL, apiURL: githubAPIURL,
	}, nil
}

// Exchanged é o que a troca devolve ao app. `AccessToken` só vem quando a troca é para
// excluir a conta; no login ele já foi apagado no GitHub.
type Exchanged struct {
	Ticket      string
	AccessToken string
}

// Exchange troca o código pelo token, lê quem é a pessoa e devolve o bilhete.
//
// `nonceHash` é o SHA-256 (hex) do nonce que o app guarda cru; o bilhete o carrega, e
// `/auth/social` confere o cru contra ele. `keepToken` deixa o access token vivo e o
// devolve, para a exclusão revogar a autorização com ele.
func (g *GitHub) Exchange(ctx context.Context, code, verifier, nonceHash string, keepToken bool) (Exchanged, error) {
	if code == "" || len(code) > maxGitHubCodeLen {
		return Exchanged{}, errors.New("código vazio ou grande demais")
	}
	if len(verifier) < minCodeVerifierLen || len(verifier) > maxCodeVerifierLen {
		return Exchanged{}, errors.New("verifier fora do tamanho")
	}
	if !isHexSHA256(nonceHash) {
		return Exchanged{}, errors.New("hash do nonce inválido")
	}

	ctx, cancel := context.WithTimeout(ctx, 3*fetchTimeout)
	defer cancel()

	token, err := g.exchangeCode(ctx, code, verifier)
	if err != nil {
		return Exchanged{}, fmt.Errorf("troca do código: %w", err)
	}
	// Daqui em diante o token existe no GitHub. Se algo falhar, ele sai junto; não
	// sobra token vivo de um login que não aconteceu.
	fail := func(err error) (Exchanged, error) {
		if derr := g.deleteToken(context.WithoutCancel(ctx), token); derr != nil {
			err = fmt.Errorf("%w (token não apagado: %v)", err, derr)
		}
		return Exchanged{}, err
	}

	subject, err := g.userID(ctx, token)
	if err != nil {
		return fail(fmt.Errorf("leitura do usuário: %w", err))
	}
	email, verified, err := g.primaryEmail(ctx, token)
	if err != nil {
		return fail(fmt.Errorf("leitura do e-mail: %w", err))
	}

	ticket, err := g.signTicket(subject, email, verified, nonceHash, time.Now())
	if err != nil {
		return fail(err)
	}
	if keepToken {
		return Exchanged{Ticket: ticket, AccessToken: token}, nil
	}
	// O contexto não morre com o pedido: o app ter desistido agora não pode deixar o
	// token vivo.
	if err := g.deleteToken(context.WithoutCancel(ctx), token); err != nil {
		// O login segue: o token é da própria pessoa e só lê o e-mail. Fica no log para
		// saber se virou regra.
		return Exchanged{Ticket: ticket}, fmt.Errorf("%w: %v", ErrTokenNotDeleted, err)
	}
	return Exchanged{Ticket: ticket}, nil
}

// ErrTokenNotDeleted diz que a troca deu certo e o bilhete vale, mas o access token
// continuou vivo no GitHub.
var ErrTokenNotDeleted = errors.New("access token do GitHub não apagado")

// Verify confere o bilhete que o próprio servidor assinou na troca (ADR 0019).
func (g *GitHub) Verify(ctx context.Context, ticket, rawNonce string) (Identity, error) {
	if ticket == "" || len(ticket) > maxIDTokenLen {
		return Identity{}, errors.New("bilhete vazio ou grande demais")
	}
	if len(rawNonce) < minNonceLen || len(rawNonce) > maxNonceLen {
		return Identity{}, errors.New("nonce fora do tamanho")
	}

	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(ticket, claims, func(*jwt.Token) (any, error) {
		return g.ticketKey, nil
	},
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithIssuer(githubTicketIssuer),
		jwt.WithAudience(githubTicketAudience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(clockSkew),
	)
	if err != nil {
		return Identity{}, err
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		return Identity{}, errors.New("bilhete sem sub")
	}
	tokenNonce, _ := claims["nonce"].(string)
	if subtle.ConstantTimeCompare([]byte(tokenNonce), []byte(hashNonce(rawNonce))) != 1 {
		return Identity{}, errors.New("nonce não confere")
	}
	issuedAt, err := claims.GetIssuedAt()
	if err != nil || issuedAt == nil {
		return Identity{}, errors.New("bilhete sem iat")
	}
	email, _ := claims["email"].(string)
	verified, _ := claims["email_verified"].(bool)
	return Identity{Subject: sub, Email: email, EmailVerified: verified, IssuedAt: issuedAt.Time}, nil
}

// Revoke tira o LogN dos apps autorizados da pessoa no GitHub. `accessToken` é o que a
// troca de exclusão devolveu; ele tem de ser deste app e de `subject`, o id que provou a
// posse da conta, ou nada é revogado.
func (g *GitHub) Revoke(ctx context.Context, accessToken, subject string) error {
	if accessToken == "" || subject == "" {
		return errors.New("sem access token ou sub")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*fetchTimeout)
	defer cancel()

	// O GitHub diz de que app e de que usuário o token é. Um token de outra pessoa, ou
	// emitido para outro app, para aqui.
	var check struct {
		App struct {
			ClientID string `json:"client_id"`
		} `json:"app"`
		User struct {
			ID int64 `json:"id"`
		} `json:"user"`
	}
	if err := g.appCall(ctx, http.MethodPost, "/token", accessToken, http.StatusOK, &check); err != nil {
		return fmt.Errorf("conferência do token: %w", err)
	}
	if check.App.ClientID != g.clientID {
		return errors.New("token de outro app; nada revogado")
	}
	if got := strconv.FormatInt(check.User.ID, 10); subtle.ConstantTimeCompare([]byte(got), []byte(subject)) != 1 {
		return errors.New("token de outro usuário; nada revogado")
	}
	if err := g.appCall(ctx, http.MethodDelete, "/grant", accessToken, http.StatusNoContent, nil); err != nil {
		return fmt.Errorf("revogação: %w", err)
	}
	return nil
}

// Discard apaga o access token de uma troca de exclusão que não chegou a revogar: a
// exclusão foi recusada depois de o token sair do GitHub, e ele não expira sozinho.
// Não mexe na autorização, que continua como estava.
func (g *GitHub) Discard(ctx context.Context, accessToken string) error {
	if accessToken == "" {
		return nil
	}
	return g.deleteToken(ctx, accessToken)
}

func (g *GitHub) signTicket(subject, email string, verified bool, nonceHash string, now time.Time) (string, error) {
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss":            githubTicketIssuer,
		"aud":            githubTicketAudience,
		"sub":            subject,
		"email":          email,
		"email_verified": verified,
		"nonce":          nonceHash,
		"iat":            now.Unix(),
		"exp":            now.Add(githubTicketTTL).Unix(),
	})
	return t.SignedString(g.ticketKey)
}

// exchangeCode troca o código pelo access token. O GitHub responde 200 também no erro,
// com o motivo em `error`.
func (g *GitHub) exchangeCode(ctx context.Context, code, verifier string) (string, error) {
	form := url.Values{
		"client_id":     {g.clientID},
		"client_secret": {g.clientSecret},
		"code":          {code},
		"redirect_uri":  {g.redirectURI},
		"code_verifier": {verifier},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.oauthURL+"/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	var out struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Error       string `json:"error"`
	}
	if err := g.do(req, http.StatusOK, &out); err != nil {
		return "", err
	}
	if out.Error != "" {
		// `bad_verification_code`, `incorrect_client_credentials`...: só um código, sem
		// dado do jogador.
		return "", fmt.Errorf("GitHub recusou: %s", clip(out.Error))
	}
	if out.AccessToken == "" || !strings.EqualFold(out.TokenType, "bearer") {
		return "", errors.New("resposta sem access token")
	}
	return out.AccessToken, nil
}

func (g *GitHub) userID(ctx context.Context, token string) (string, error) {
	var user struct {
		ID int64 `json:"id"`
	}
	if err := g.userCall(ctx, "/user", token, &user); err != nil {
		return "", err
	}
	if user.ID <= 0 {
		return "", errors.New("usuário sem id")
	}
	return strconv.FormatInt(user.ID, 10), nil
}

// primaryEmail devolve o e-mail principal e se ele está verificado. Só o principal
// conta: um secundário verificado é endereço que a pessoa acrescentou, não o da conta.
func (g *GitHub) primaryEmail(ctx context.Context, token string) (string, bool, error) {
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := g.userCall(ctx, "/user/emails", token, &emails); err != nil {
		return "", false, err
	}
	for _, e := range emails {
		if e.Primary {
			return e.Email, e.Verified, nil
		}
	}
	return "", false, nil
}

// deleteToken apaga só este access token; a autorização do app continua, e o próximo
// login não pede consentimento de novo.
func (g *GitHub) deleteToken(ctx context.Context, token string) error {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	return g.appCall(ctx, http.MethodDelete, "/token", token, http.StatusNoContent, nil)
}

// userCall chama a API em nome da pessoa, com o access token dela.
func (g *GitHub) userCall(ctx context.Context, path, token string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.apiURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return g.do(req, http.StatusOK, out)
}

// appCall chama `/applications/{client_id}{path}` em nome do app, com o secret, sobre
// um access token.
func (g *GitHub) appCall(ctx context.Context, method, path, token string, want int, out any) error {
	body, err := json.Marshal(map[string]string{"access_token": token})
	if err != nil {
		return err
	}
	endpoint := g.apiURL + "/applications/" + url.PathEscape(g.clientID) + path
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.SetBasicAuth(g.clientID, g.clientSecret)
	req.Header.Set("Content-Type", "application/json")
	return g.do(req, want, out)
}

func (g *GitHub) do(req *http.Request, want int, out any) error {
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/vnd.github+json")
	}
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	// O GitHub recusa pedido sem User-Agent.
	req.Header.Set("User-Agent", "logn-backend")
	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != want {
		// A mensagem de erro do GitHub não traz dado do jogador; vai cortada para o log.
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("status %d: %s", resp.StatusCode, clip(string(body)))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(io.LimitReader(resp.Body, maxGitHubBodyBytes)).Decode(out)
}

func isHexSHA256(s string) bool {
	if len(s) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil && strings.ToLower(s) == s
}
