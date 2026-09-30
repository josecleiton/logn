// Package googleplay verifica compras do Google Play pela Google Play Developer API
// (ADR 0022), sem SDK: REST por `net/http`, com o token da conta de serviço do Cloud
// Run tirado do servidor de metadados. Não há chave nem segredo: a conta é convidada no
// Play Console, com permissão só de ver dados financeiros e gerenciar pedidos.
//
// O que o servidor aceita da loja mora aqui, num lugar só, como a validação do JWS da
// Apple mora em `internal/storekit`.
package googleplay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultBaseURL é a API de produção. Os testes trocam por um servidor local.
	DefaultBaseURL = "https://androidpublisher.googleapis.com"
	// O escopo que a API pede. O token do Cloud Run sai com `cloud-platform` se não
	// pedirmos este.
	scope = "https://www.googleapis.com/auth/androidpublisher"
	// O token da conta de serviço, do servidor de metadados do Google Cloud.
	metadataTokenURL = "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token?scopes=" + scope

	// Teto da resposta lida. Uma compra tem poucas centenas de bytes; a página de
	// compras anuladas, algumas dezenas de KB.
	maxResponse = 1 << 20
	// Páginas de compras anuladas por consulta. Mil por página: passar disto num dia é
	// sinal de problema, não de volume.
	maxVoidedPages = 20
)

var (
	// ErrNotFound é o token que a loja não reconhece: inventado, de outro app ou de
	// outro produto.
	ErrNotFound = errors.New("googleplay: compra não encontrada")
	// ErrPending é a compra que ainda não foi paga (boleto, dinheiro). Não libera nada.
	ErrPending = errors.New("googleplay: compra pendente")
	// ErrNotPurchased é a compra cancelada, consumida ou de recompensa.
	ErrNotPurchased = errors.New("googleplay: compra não vale")
	// ErrBadInput é produto ou token fora do formato. Não sai pedido para a loja.
	ErrBadInput = errors.New("googleplay: produto ou token fora do formato")
)

// O formato que o Play aceita para id de produto, e o do token de compra (base64url com
// pontos). Os dois entram no caminho da URL, e nada fora disto chega a ela.
var (
	productPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._]{0,148}$`)
	// O `regexp` do Go não repete mais de mil vezes; o tamanho sai em ValidInput.
	tokenPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

// ValidProductID diz se o id de produto vale no Play. A validação do conteúdo usa esta,
// para o catálogo e a rota nunca discordarem.
func ValidProductID(productID string) bool { return productPattern.MatchString(productID) }

// ValidInput confere produto e token antes de qualquer pedido à loja.
func ValidInput(productID, purchaseToken string) bool {
	return productPattern.MatchString(productID) &&
		len(purchaseToken) >= 16 && len(purchaseToken) <= 4096 && tokenPattern.MatchString(purchaseToken)
}

// TransactionKey é o que vai em `original_transaction_id`: o SHA-256 hex do token. O
// token não tem tamanho máximo documentado, e o `orderId` não vem na compra de testador
// de licença. A API de compras anuladas devolve o token, e a revogação acha a linha pelo
// mesmo hash.
func TransactionKey(purchaseToken string) string {
	sum := sha256.Sum256([]byte(purchaseToken))
	return hex.EncodeToString(sum[:])
}

// ProductPurchase é a compra de produto único como a API devolve. Campo com valor zero
// pode vir omitido: `purchaseState` ausente é comprado, e `purchaseType` ausente é
// compra de verdade — por isso ele é ponteiro.
type ProductPurchase struct {
	// O produto que a loja diz que foi comprado. A documentação diz que pode faltar;
	// quando vem, tem de ser o que o app mandou (ForProduct).
	ProductID                   string `json:"productId"`
	Quantity                    int    `json:"quantity"`
	PurchaseState               int    `json:"purchaseState"`
	ConsumptionState            int    `json:"consumptionState"`
	AcknowledgementState        int    `json:"acknowledgementState"`
	PurchaseType                *int   `json:"purchaseType"`
	OrderID                     string `json:"orderId"`
	ObfuscatedExternalAccountID string `json:"obfuscatedExternalAccountId"`
	PurchaseTimeMillis          string `json:"purchaseTimeMillis"`
	// A resposta como chegou, para o registro da compra.
	Raw json.RawMessage `json:"-"`
}

// Os ambientes gravados em `store_transactions`.
const (
	EnvironmentProduction = "Production"
	EnvironmentTest       = "Test"
)

// Environment diz se a compra vale e em que ambiente.
//
//   - comprada (`purchaseState` 0), não consumida: vale;
//   - pendente (2): ErrPending, e o app espera o pagamento;
//   - cancelada (1), consumida, ou de recompensa (`purchaseType` 2): ErrNotPurchased;
//   - `purchaseType` 0 é testador de licença: vale, em `Test` (ADR 0022). Só as contas
//     cadastradas no Play Console conseguem fazê-la;
//   - `purchaseType` 1 é código promocional: compra de verdade, de graça.
func (p ProductPurchase) Environment() (string, error) {
	switch p.PurchaseState {
	case 0:
	case 2:
		return "", ErrPending
	default:
		return "", ErrNotPurchased
	}
	if p.ConsumptionState != 0 {
		return "", ErrNotPurchased
	}
	if p.PurchaseType == nil {
		return EnvironmentProduction, nil
	}
	switch *p.PurchaseType {
	case 0:
		return EnvironmentTest, nil
	case 1:
		return EnvironmentProduction, nil
	default:
		return "", ErrNotPurchased
	}
}

// ForProduct diz se a compra é do produto pedido. O id do produto vai no caminho da
// consulta, e a loja recusa token de outro produto; mas não é isso que a documentação
// promete, e um token de um item barato do mesmo app não pode abrir uma trilha. Com o
// `productId` na resposta, ele manda. Quantidade acima de um também não é compra de
// trilha.
func (p ProductPurchase) ForProduct(productID string) bool {
	if p.ProductID != "" && p.ProductID != productID {
		return false
	}
	return p.Quantity <= 1
}

// NeedsAcknowledge diz se a compra ainda não foi reconhecida. Sem reconhecimento em 3
// dias, o Play estorna sozinho.
func (p ProductPurchase) NeedsAcknowledge() bool { return p.AcknowledgementState == 0 }

// VoidedPurchase é uma compra anulada: reembolso, estorno ou cancelamento.
type VoidedPurchase struct {
	PurchaseToken    string `json:"purchaseToken"`
	VoidedTimeMillis string `json:"voidedTimeMillis"`
	// 5 fraude, 6 fraude amigável (quem comprou pediu estorno ao cartão), 7 estorno.
	VoidedReason int `json:"voidedReason"`
}

// Fraud diz se a anulação foi por fraude ou estorno contestado, que vira `fraud` em
// `revoked_transactions` em vez de `refund`.
func (v VoidedPurchase) Fraud() bool {
	return v.VoidedReason == 5 || v.VoidedReason == 6 || v.VoidedReason == 7
}

// VoidedAtMs é a hora da anulação, em ms, ou 0 se ela não vier.
func (v VoidedPurchase) VoidedAtMs() int64 {
	ms, _ := strconv.ParseInt(v.VoidedTimeMillis, 10, 64)
	return ms
}

// TokenSource entrega o token de acesso da API.
type TokenSource interface {
	Token(ctx context.Context) (string, error)
}

// Client fala com a API para um app.
type Client struct {
	packageName string
	baseURL     string
	http        *http.Client
	tokens      TokenSource
}

// packagePattern é o formato do nome de pacote Android.
var packagePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)

// NewClient monta o cliente de um pacote. `tokens` nulo usa o servidor de metadados.
func NewClient(packageName, baseURL string, tokens TokenSource) (*Client, error) {
	if !packagePattern.MatchString(packageName) {
		return nil, fmt.Errorf("googleplay: nome de pacote inválido: %q", packageName)
	}
	httpClient := &http.Client{Timeout: 10 * time.Second}
	if tokens == nil {
		tokens = &MetadataTokenSource{http: httpClient, url: metadataTokenURL}
	}
	return &Client{packageName: packageName, baseURL: baseURL, http: httpClient, tokens: tokens}, nil
}

func (c *Client) productURL(productID, purchaseToken, suffix string) string {
	return c.baseURL + "/androidpublisher/v3/applications/" + url.PathEscape(c.packageName) +
		"/purchases/products/" + url.PathEscape(productID) + "/tokens/" + url.PathEscape(purchaseToken) + suffix
}

// Product lê a compra de um produto único.
func (c *Client) Product(ctx context.Context, productID, purchaseToken string) (ProductPurchase, error) {
	if !ValidInput(productID, purchaseToken) {
		return ProductPurchase{}, ErrBadInput
	}
	body, err := c.do(ctx, http.MethodGet, c.productURL(productID, purchaseToken, ""))
	if err != nil {
		return ProductPurchase{}, err
	}
	var p ProductPurchase
	if err := json.Unmarshal(body, &p); err != nil {
		return ProductPurchase{}, fmt.Errorf("googleplay: resposta da compra ilegível: %w", err)
	}
	p.Raw = body
	return p, nil
}

// Acknowledge reconhece a compra. Reconhecer de novo responde erro na API; quem chama
// só reconhece o que NeedsAcknowledge disse.
func (c *Client) Acknowledge(ctx context.Context, productID, purchaseToken string) error {
	if !ValidInput(productID, purchaseToken) {
		return ErrBadInput
	}
	_, err := c.do(ctx, http.MethodPost, c.productURL(productID, purchaseToken, ":acknowledge"))
	return err
}

// Voided lista as compras de produto único anuladas desde `since`. A API guarda 30 dias
// para trás.
func (c *Client) Voided(ctx context.Context, since time.Time) ([]VoidedPurchase, error) {
	var out []VoidedPurchase
	page := ""
	for i := 0; i < maxVoidedPages; i++ {
		q := url.Values{}
		q.Set("startTime", strconv.FormatInt(since.UnixMilli(), 10))
		q.Set("type", "0")
		q.Set("maxResults", "1000")
		if page != "" {
			q.Set("token", page)
		}
		body, err := c.do(ctx, http.MethodGet, c.baseURL+"/androidpublisher/v3/applications/"+
			url.PathEscape(c.packageName)+"/purchases/voidedpurchases?"+q.Encode())
		if err != nil {
			return nil, err
		}
		var resp struct {
			VoidedPurchases []VoidedPurchase `json:"voidedPurchases"`
			TokenPagination struct {
				NextPageToken string `json:"nextPageToken"`
			} `json:"tokenPagination"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("googleplay: resposta das anuladas ilegível: %w", err)
		}
		out = append(out, resp.VoidedPurchases...)
		if resp.TokenPagination.NextPageToken == "" {
			return out, nil
		}
		page = resp.TokenPagination.NextPageToken
	}
	return nil, fmt.Errorf("googleplay: mais de %d páginas de compras anuladas", maxVoidedPages)
}

func (c *Client) do(ctx context.Context, method, target string) ([]byte, error) {
	token, err := c.tokens.Token(ctx)
	if err != nil {
		return nil, err
	}
	var body io.Reader
	if method == http.MethodPost {
		body = strings.NewReader("{}")
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		// O `*url.Error` repete a URL inteira, e ela leva o token de compra. Só a causa
		// vai adiante: o erro acaba no log.
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return nil, fmt.Errorf("googleplay: pedido falhou: %w", err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 == 2 {
		return out, nil
	}
	if resp.StatusCode == http.StatusUnauthorized {
		// Token de acesso recusado antes de vencer: o próximo pedido busca outro.
		if m, ok := c.tokens.(interface{ forget() }); ok {
			m.forget()
		}
	}
	reason := apiReason(out)
	if tokenRejected(resp.StatusCode, reason) {
		return nil, ErrNotFound
	}
	return nil, &APIError{Status: resp.StatusCode, Reason: reason}
}

// APIError é a resposta fora de 2xx que não diz "este token não vale": configuração
// nossa (pacote errado, conta sem permissão, API desligada) ou a loja fora. Quem chama
// responde 502, e o app tenta de novo, em vez de ouvir que a compra é inválida.
type APIError struct {
	Status int
	Reason string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("googleplay: a API respondeu %d (%s)", e.Status, e.Reason)
}

// apiReason tira o `reason` do corpo de erro da API. Só ele: a mensagem pode repetir
// parte do pedido.
func apiReason(body []byte) string {
	var e struct {
		Error struct {
			Errors []struct {
				Reason string `json:"reason"`
			} `json:"errors"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &e) != nil || len(e.Error.Errors) == 0 {
		return ""
	}
	r := e.Error.Errors[0].Reason
	if !reasonPattern.MatchString(r) {
		return "?"
	}
	return r
}

var reasonPattern = regexp.MustCompile(`^[A-Za-z]{1,64}$`)

// tokenRejected diz se a resposta é a do token que não vale: inventado, de outro
// produto, vencido. 410 é sempre isso. 400 e 404 só com o motivo do token, porque 404
// também é pacote que não existe (PLAY_PACKAGE_NAME errado), e aí a culpa é nossa.
func tokenRejected(status int, reason string) bool {
	switch status {
	case http.StatusGone:
		return true
	case http.StatusBadRequest, http.StatusNotFound:
		switch reason {
		case "invalid", "purchaseTokenNotFound", "purchaseTokenDoesNotMatchProductId",
			"purchaseTokenDoesNotMatchPackageName", "notFound":
			return true
		}
	}
	return false
}

// MetadataTokenSource pega o token da conta de serviço no servidor de metadados e o
// guarda até perto de vencer.
type MetadataTokenSource struct {
	http *http.Client
	url  string

	mu      sync.Mutex
	token   string
	expires time.Time
}

func (m *MetadataTokenSource) forget() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.token = ""
}

func (m *MetadataTokenSource) Token(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.token != "" && time.Now().Before(m.expires) {
		return m.token, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Metadata-Flavor", "Google")
	resp, err := m.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("googleplay: servidor de metadados: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("googleplay: servidor de metadados respondeu %d", resp.StatusCode)
	}
	var t struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponse)).Decode(&t); err != nil || t.AccessToken == "" {
		return "", errors.New("googleplay: token do servidor de metadados ilegível")
	}
	m.token = t.AccessToken
	// Um minuto de folga para o token não vencer no meio do pedido.
	m.expires = time.Now().Add(time.Duration(t.ExpiresIn)*time.Second - time.Minute)
	return m.token, nil
}
