package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/josecleiton/logn/backend/internal/domain"
	"github.com/josecleiton/logn/backend/internal/locale"
)

const (
	testLanding = "https://landing.example.com"
	testAPI     = "https://api.example.com"
)

// fakeWaitlistMailer guarda o que seria enviado, em `sent`.
type fakeWaitlistMailer struct {
	mu   sync.Mutex
	sent chan waitlistMail
	fail bool
}

type waitlistMail struct{ to, lang, confirm, leave string }

func newFakeWaitlistMailer() *fakeWaitlistMailer {
	return &fakeWaitlistMailer{sent: make(chan waitlistMail, 16)}
}

func (m *fakeWaitlistMailer) SendWaitlistConfirmation(to, lang, confirm, leave string) error {
	m.sent <- waitlistMail{to, lang, confirm, leave}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return fmt.Errorf("smtp fora")
	}
	return nil
}

func (m *fakeWaitlistMailer) next(t *testing.T) waitlistMail {
	t.Helper()
	select {
	case mail := <-m.sent:
		return mail
	case <-time.After(2 * time.Second):
		t.Fatal("nenhum e-mail de confirmação saiu")
		return waitlistMail{}
	}
}

func (m *fakeWaitlistMailer) none(t *testing.T) {
	t.Helper()
	select {
	case mail := <-m.sent:
		t.Fatalf("e-mail inesperado para %s", mail.to)
	case <-time.After(100 * time.Millisecond):
	}
}

type waitlistHarness struct {
	t      *testing.T
	repo   *domain.Repository
	mailer *fakeWaitlistMailer
	mux    http.Handler
}

func newWaitlistHarness(t *testing.T) *waitlistHarness {
	pool := setupTestDB(t)
	repo := domain.NewRepository(pool)
	mailer := newFakeWaitlistMailer()
	s := &Server{
		repo:           repo,
		waitlist:       &WaitlistConfig{LandingOrigin: testLanding, APIOrigin: testAPI},
		waitlistMailer: mailer,
	}
	mux := http.NewServeMux()
	s.registerWaitlistRoutes(mux)
	// Sem fila, a caixa de saída envia na hora, quando o pedido termina (ADR 0026).
	return &waitlistHarness{t: t, repo: repo, mailer: mailer, mux: s.withOutbox(mux)}
}

// join posta o formulário de um IP, como o navegador na landing: `Origin: null` (a
// landing sai com `no-referrer`) e `Sec-Fetch-Site: same-site`. Cada teste usa o seu IP,
// para não gastar o balde do outro.
func (h *waitlistHarness) join(ip string, form url.Values) *httptest.ResponseRecorder {
	return h.joinWith(ip, form, map[string]string{"Origin": "null", "Sec-Fetch-Site": "same-site"})
}

func (h *waitlistHarness) joinWith(ip string, form url.Values, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/waitlist", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.RemoteAddr = ip + ":1234"
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, req)
	return rec
}

func (h *waitlistHarness) do(method, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	req.RemoteAddr = "192.0.2.200:1234"
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, req)
	return rec
}

func expectRedirect(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d, esperava 303 para %s", rec.Code, want)
	}
	if got := rec.Header().Get("Location"); got != want {
		t.Fatalf("Location %q, esperava %q", got, want)
	}
}

// pathOf tira o caminho com a query de um link do e-mail, para o mux de teste.
func pathOf(t *testing.T, link string) string {
	t.Helper()
	if !strings.HasPrefix(link, testAPI+"/api/v1/waitlist/") {
		t.Fatalf("link fora da API: %s", link)
	}
	return strings.TrimPrefix(link, testAPI)
}

func cleanupWaitlist(t *testing.T, repo *domain.Repository, addr string) {
	t.Cleanup(func() {
		id, _, _ := repo.JoinWaitlist(context.Background(), addr, "pt-BR")
		if id != "" {
			repo.LeaveWaitlist(context.Background(), id)
		}
	})
}

func TestWaitlistJoinConfirmLeave(t *testing.T) {
	h := newWaitlistHarness(t)
	addr := fmt.Sprintf("Waitlist-Flow-%d@Example.com", time.Now().UnixNano())
	normalized := strings.ToLower(addr)

	rec := h.join("192.0.2.10", url.Values{"email": {addr}, "locale": {"en"}, "website": {""}})
	expectRedirect(t, rec, testLanding+"/en/waitlist/thanks/")
	mail := h.mailer.next(t)
	if mail.to != normalized || mail.lang != "en" {
		t.Fatalf("e-mail para %q em %q", mail.to, mail.lang)
	}
	if strings.Contains(mail.confirm, "@") || strings.Contains(mail.leave, "@") {
		t.Error("o link do e-mail leva o endereço")
	}

	// Inscrever de novo responde igual e não manda nada: a rota não diz quem já está.
	expectRedirect(t, h.join("192.0.2.10", url.Values{"email": {addr}, "locale": {"en"}}), testLanding+"/en/waitlist/thanks/")
	h.mailer.none(t)

	// O GET do link não confirma: filtro de e-mail abre todo link.
	confirm := pathOf(t, mail.confirm)
	page := h.do(http.MethodGet, confirm)
	if page.Code != http.StatusOK {
		t.Fatalf("página de confirmação: %d", page.Code)
	}
	csp := page.Header().Get("Content-Security-Policy")
	for _, want := range []string{"default-src 'none'", "form-action 'self' " + testLanding, "frame-ancestors 'none'", "'" + waitlistPageCSSHash + "'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP sem %q: %s", want, csp)
		}
	}
	if strings.Contains(csp, "unsafe-inline") {
		t.Error("CSP com unsafe-inline")
	}
	if page.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Error("página com token sem Referrer-Policy: no-referrer")
	}
	if !strings.Contains(page.Body.String(), `lang="en"`) || !strings.Contains(page.Body.String(), `method="post"`) {
		t.Errorf("página sem a língua ou o formulário:\n%s", page.Body.String())
	}
	id, _ := domain.ParseWaitlistToken(strings.TrimPrefix(confirm, "/api/v1/waitlist/confirm?t="), domain.WaitlistActionConfirm)
	if e, err := h.repo.GetWaitlistEntry(context.Background(), id); err != nil || e.Confirmed {
		t.Fatalf("o GET confirmou: %+v %v", e, err)
	}

	expectRedirect(t, h.do(http.MethodPost, confirm), testLanding+"/en/waitlist/confirmed/")
	if e, _ := h.repo.GetWaitlistEntry(context.Background(), id); !e.Confirmed {
		t.Fatal("o POST não confirmou")
	}
	// O link de confirmação aberto de novo vai direto para "confirmada".
	expectRedirect(t, h.do(http.MethodGet, confirm), testLanding+"/en/waitlist/confirmed/")

	leave := pathOf(t, mail.leave)
	if page := h.do(http.MethodGet, leave); page.Code != http.StatusOK {
		t.Fatalf("página de saída: %d", page.Code)
	}
	expectRedirect(t, h.do(http.MethodPost, leave), testLanding+"/en/waitlist/left/")
	if _, err := h.repo.GetWaitlistEntry(context.Background(), id); err == nil {
		t.Fatal("a saída não apagou a inscrição")
	}
	// O cliente de e-mail que repete o `List-Unsubscribe-Post` também vê "saiu".
	expectRedirect(t, h.do(http.MethodPost, leave), testLanding+"/waitlist/left/")
	// E o link da inscrição apagada já não abre página.
	expectRedirect(t, h.do(http.MethodGet, confirm), testLanding+"/waitlist/error/")
}

func TestWaitlistHoneypot(t *testing.T) {
	h := newWaitlistHarness(t)
	addr := fmt.Sprintf("waitlist-bot-%d@example.com", time.Now().UnixNano())
	cleanupWaitlist(t, h.repo, addr)

	rec := h.join("192.0.2.11", url.Values{"email": {addr}, "locale": {"es"}, "website": {"https://spam.example.com"}})
	expectRedirect(t, rec, testLanding+"/es/waitlist/thanks/")
	h.mailer.none(t)
	// Nada foi gravado: a inscrição de verdade ainda sai como nova.
	if _, send, _ := h.repo.JoinWaitlist(context.Background(), addr, "es"); !send {
		t.Error("a isca gravou a inscrição")
	}
}

// Página de outro site postando o formulário pelo navegador de quem a visita: cada
// visitante é um IP, e o limite por IP não vê.
func TestWaitlistRejectsForeignSites(t *testing.T) {
	h := newWaitlistHarness(t)
	addr := fmt.Sprintf("waitlist-csrf-%d@example.com", time.Now().UnixNano())
	cleanupWaitlist(t, h.repo, addr)
	form := url.Values{"email": {addr}, "locale": {"en"}}

	for name, headers := range map[string]map[string]string{
		"outro site":           {"Origin": "https://evil.example.com", "Sec-Fetch-Site": "cross-site"},
		"null de outro site":   {"Origin": "null", "Sec-Fetch-Site": "cross-site"},
		"sem nenhum cabeçalho": {},
		"origem parecida":      {"Origin": testLanding + ".evil.example.com"},
	} {
		t.Run(name, func(t *testing.T) {
			expectRedirect(t, h.joinWith("192.0.2.16", form, headers), testLanding+"/en/waitlist/error/")
		})
	}
	h.mailer.none(t)

	// A origem exata da landing passa, mesmo sem Sec-Fetch-Site.
	expectRedirect(t, h.joinWith("192.0.2.16", form, map[string]string{"Origin": testLanding}), testLanding+"/en/waitlist/thanks/")
	h.mailer.next(t)
}

func TestWaitlistRejectsBadInput(t *testing.T) {
	h := newWaitlistHarness(t)

	for name, form := range map[string]url.Values{
		"sem e-mail":          {"locale": {"en"}},
		"e-mail inválido":     {"email": {"not-an-email"}, "locale": {"en"}},
		"nome de exibição":    {"email": {"Eve <eve@example.com>"}, "locale": {"en"}},
		"quebra de cabeçalho": {"email": {"eve@example.com\r\nBcc: x@example.com"}, "locale": {"en"}},
	} {
		t.Run(name, func(t *testing.T) {
			expectRedirect(t, h.join("192.0.2.12", form), testLanding+"/en/waitlist/error/")
		})
	}
	h.mailer.none(t)

	// Língua fora da lista cai na padrão, e não vira caminho.
	expectRedirect(t, h.join("192.0.2.12", url.Values{"email": {"x"}, "locale": {"../../evil"}}), testLanding+"/waitlist/error/")

	// Corpo acima do teto.
	big := url.Values{"email": {"a@example.com"}, "locale": {"en"}, "website": {strings.Repeat("x", waitlistBodyLimit)}}
	expectRedirect(t, h.join("192.0.2.12", big), testLanding+"/waitlist/error/")
	h.mailer.none(t)
}

func TestWaitlistRateLimit(t *testing.T) {
	h := newWaitlistHarness(t)
	ip := "192.0.2.13"
	for i := 0; i < 10; i++ {
		rec := h.join(ip, url.Values{"email": {"not-an-email"}, "locale": {"en"}})
		expectRedirect(t, rec, testLanding+"/en/waitlist/error/")
	}
	addr := fmt.Sprintf("waitlist-limit-%d@example.com", time.Now().UnixNano())
	cleanupWaitlist(t, h.repo, addr)
	// O décimo primeiro, mesmo válido, para no balde do IP.
	expectRedirect(t, h.join(ip, url.Values{"email": {addr}, "locale": {"en"}}), testLanding+"/en/waitlist/error/")
	h.mailer.none(t)
	// Outro IP passa.
	expectRedirect(t, h.join("192.0.2.14", url.Values{"email": {addr}, "locale": {"en"}}), testLanding+"/en/waitlist/thanks/")
	h.mailer.next(t)
}

func TestWaitlistTokens(t *testing.T) {
	h := newWaitlistHarness(t)
	addr := fmt.Sprintf("waitlist-token-%d@example.com", time.Now().UnixNano())
	cleanupWaitlist(t, h.repo, addr)
	id, _, err := h.repo.JoinWaitlist(context.Background(), addr, "pt-BR")
	if err != nil {
		t.Fatal(err)
	}
	confirm := domain.WaitlistToken(id, domain.WaitlistActionConfirm)
	leave := domain.WaitlistToken(id, domain.WaitlistActionLeave)
	// Troca o último dígito por outro, qualquer que seja ele.
	last := "0"
	if strings.HasSuffix(confirm, "0") {
		last = "1"
	}
	tampered := confirm[:len(confirm)-1] + last

	for name, target := range map[string]string{
		"sem token":                   "/api/v1/waitlist/confirm",
		"token adulterado":            "/api/v1/waitlist/confirm?t=" + url.QueryEscape(tampered),
		"token de saída no confirmar": "/api/v1/waitlist/confirm?t=" + url.QueryEscape(leave),
		"token de confirmar na saída": "/api/v1/waitlist/leave?t=" + url.QueryEscape(confirm),
	} {
		t.Run(name, func(t *testing.T) {
			expectRedirect(t, h.do(http.MethodGet, target), testLanding+"/waitlist/error/")
			expectRedirect(t, h.do(http.MethodPost, target), testLanding+"/waitlist/error/")
		})
	}
	e, err := h.repo.GetWaitlistEntry(context.Background(), id)
	if err != nil || e.Confirmed {
		t.Fatalf("token inválido mudou a inscrição: %+v %v", e, err)
	}
}

func TestWaitlistMailFailureReleasesCooldown(t *testing.T) {
	h := newWaitlistHarness(t)
	h.mailer.fail = true
	addr := fmt.Sprintf("waitlist-smtp-%d@example.com", time.Now().UnixNano())
	cleanupWaitlist(t, h.repo, addr)

	expectRedirect(t, h.join("192.0.2.15", url.Values{"email": {addr}, "locale": {"pt-BR"}}), testLanding+"/waitlist/thanks/")
	h.mailer.next(t)
	// Sem fila, o envio é a última tentativa, e a falha libera o reenvio.
	_, send, err := h.repo.JoinWaitlist(context.Background(), addr, "pt-BR")
	if err != nil {
		t.Fatal(err)
	}
	if !send {
		t.Fatal("o envio falhou e o reenvio continuou travado")
	}
}

func TestWaitlistExpiredLinkGoesToError(t *testing.T) {
	h := newWaitlistHarness(t)
	addr := fmt.Sprintf("waitlist-expired-%d@example.com", time.Now().UnixNano())
	cleanupWaitlist(t, h.repo, addr)
	id, _, err := h.repo.JoinWaitlist(context.Background(), addr, "es")
	if err != nil {
		t.Fatal(err)
	}
	pool := setupTestDB(t)
	pool.Exec(context.Background(), `UPDATE waitlist_entries SET created_at = created_at - interval '8 days' WHERE id = $1`, id)

	target := "/api/v1/waitlist/confirm?t=" + url.QueryEscape(domain.WaitlistToken(id, domain.WaitlistActionConfirm))
	expectRedirect(t, h.do(http.MethodGet, target), testLanding+"/waitlist/error/")
	expectRedirect(t, h.do(http.MethodPost, target), testLanding+"/waitlist/error/")
}

// O hash da CSP tem de ser o do `<style>` que sai na página, byte a byte: um espaço a
// mais no template e a página perderia o estilo sem teste nenhum cair.
func TestWaitlistPageStyleMatchesCSP(t *testing.T) {
	rec := httptest.NewRecorder()
	writeWaitlistPage(rec, locale.En, domain.WaitlistActionLeave, "/api/v1/waitlist/leave?t=x", testLanding)
	body := rec.Body.String()
	start, end := strings.Index(body, "<style>"), strings.Index(body, "</style>")
	if start < 0 || end < start {
		t.Fatal("página sem <style>")
	}
	sum := sha256.Sum256([]byte(body[start+len("<style>") : end]))
	want := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, want) {
		t.Errorf("CSP sem o hash do estilo servido %s: %s", want, csp)
	}
}

// Língua nova no locale sem caminho na landing mandaria para a página em português
// sem erro nenhum.
func TestWaitlistLandingCoversEveryLocale(t *testing.T) {
	for _, l := range locale.Supported {
		if _, ok := landingPath[l]; !ok {
			t.Errorf("landingPath sem %s", l)
		}
		if _, ok := waitlistPageCopies[l]; !ok {
			t.Errorf("waitlistPageCopies sem %s", l)
		}
	}
}

func TestRedactEmails(t *testing.T) {
	err := fmt.Errorf("550 5.1.1 <victim.name+tag@mail.example.com>: Recipient address rejected; from=noreply@logn.sh")
	got := redactEmails(err)
	if strings.Contains(got, "@") {
		t.Errorf("sobrou endereço: %s", got)
	}
	if !strings.Contains(got, "550 5.1.1") {
		t.Errorf("o código do SMTP sumiu: %s", got)
	}
}

func TestWaitlistRoutesOffWithoutConfig(t *testing.T) {
	h, err := New(Deps{})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/waitlist", strings.NewReader("email=a%40example.com"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("lista desligada respondeu %d", rec.Code)
	}
}

func TestWaitlistConfigFromEnv(t *testing.T) {
	env := func(landing, api string) func(string) string {
		return func(k string) string {
			switch k {
			case "WAITLIST_LANDING_ORIGIN":
				return landing
			case "WAITLIST_API_ORIGIN":
				return api
			}
			return ""
		}
	}
	if c, err := WaitlistConfigFromEnv(env("", "")); c != nil || err != nil {
		t.Errorf("sem as duas: %+v %v", c, err)
	}
	if c, err := WaitlistConfigFromEnv(env(testLanding, testAPI)); err != nil || c.APIOrigin != testAPI {
		t.Errorf("as duas certas: %+v %v", c, err)
	}
	for _, bad := range [][2]string{
		{testLanding, ""},
		{"", testAPI},
		{"http://landing.example.com", testAPI},
		{testLanding + "/", testAPI},
		{testLanding, testAPI + "/api"},
		{testLanding, "https://logn-abc.a.run.app"},
		{"https://landing.example.com?x=1", testAPI},
	} {
		if _, err := WaitlistConfigFromEnv(env(bad[0], bad[1])); err == nil {
			t.Errorf("aceitou landing=%q api=%q", bad[0], bad[1])
		}
	}
}
