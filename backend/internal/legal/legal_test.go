package legal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// fakeStore serve os fragmentos de testdata/. O texto real vive no repositório de
// conteúdo, que é privado: os testes daqui não podem depender dele.
type fakeStore struct {
	docs map[string]Document
	err  error
}

func newFakeStore(t *testing.T, locales ...string) *fakeStore {
	t.Helper()
	s := &fakeStore{docs: map[string]Document{}}
	for _, kind := range []Kind{Terms, Privacy} {
		for _, locale := range locales {
			raw, err := os.ReadFile(filepath.Join("testdata", fmt.Sprintf("%s.%s.html", kind, locale)))
			if err != nil {
				t.Fatal(err)
			}
			s.docs[string(kind)+"/"+locale] = Document{
				Kind: kind, Locale: locale, Version: 2,
				EffectiveAt: time.Date(2026, 10, 1, 3, 0, 0, 0, time.UTC),
				Body:        string(raw),
			}
		}
	}
	return s
}

func (s *fakeStore) Latest(_ context.Context, kind Kind, locale string) (Document, error) {
	if s.err != nil {
		return Document{}, s.err
	}
	d, ok := s.docs[string(kind)+"/"+locale]
	if !ok {
		return Document{}, ErrNotFound
	}
	return d, nil
}

func serve(t *testing.T, h http.HandlerFunc, target string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func TestNegotiate(t *testing.T) {
	cases := []struct {
		query, accept, want string
	}{
		{"es", "", Es},
		{"pt", "", PtBR},
		{"EN-us", "", En},
		{"es_MX", "", Es},
		{"fr", "", PtBR},
		{"fr", "es-MX,es;q=0.9", Es},
		{"", "en-US,en;q=0.9", En},
		{"", "de-DE,de;q=0.9", PtBR},
		{"", "de;q=0.9,es;q=0.8", Es},
		{"", "fr;q=0.2,en;q=0.9", En},
		{"", "%%%;;;q=abc", PtBR},
		{"", "", PtBR},
		{"en", "es", En},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodGet, "/legal/terms?lang="+c.query, nil)
		if c.accept != "" {
			req.Header.Set("Accept-Language", c.accept)
		}
		if got := Negotiate(req); got != c.want {
			t.Errorf("lang=%q accept=%q: got %q, want %q", c.query, c.accept, got, c.want)
		}
	}
}

func TestValidateAcceptsFixtureAndReturnsIDsInOrder(t *testing.T) {
	raw, _ := os.ReadFile("testdata/terms.pt-BR.html")
	ids, err := Validate(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(ids, ",") != "quem-oferece,trilhas-pagas" {
		t.Fatalf("ids = %v", ids)
	}
}

func TestValidateRejects(t *testing.T) {
	bad := map[string]string{
		"script":        `<article><script>alert(1)</script></article>`,
		"onclick":       `<article><p onclick="x()">a</p></article>`,
		"style attr":    `<article><p style="color:red">a</p></article>`,
		"style tag":     `<article><style>p{}</style></article>`,
		"javascript":    `<article><a href="javascript:alert(1)">a</a></article>`,
		"http":          `<article><a href="http://example.com">a</a></article>`,
		"other path":    `<article><a href="/api/v1/sync">a</a></article>`,
		"iframe":        `<article><iframe src="https://x"></iframe></article>`,
		"repeated id":   `<article><section id="a"></section><section id="a"></section></article>`,
		"id not a slug": `<article><section id="Quem Oferece"></section></article>`,
		"id elsewhere":  `<article><p id="x">a</p></article>`,
		"comment":       `<article><!-- nota --></article>`,
	}
	for name, body := range bad {
		if _, err := Validate(body); err == nil {
			t.Errorf("%s: aceitou %q", name, body)
		}
	}
}

func TestValidateSet(t *testing.T) {
	if errs := ValidateSet("testdata", false); len(errs) != 0 {
		t.Fatalf("fixtures deviam passar sem release: %v", errs)
	}
	errs := ValidateSet("testdata", true)
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "terms.pt-BR.html") {
		t.Fatalf("release devia acusar só o rascunho de terms.pt-BR: %v", errs)
	}

	dir := t.TempDir()
	for _, f := range []string{"terms.pt-BR.html", "terms.en.html", "privacy.pt-BR.html", "privacy.en.html", "privacy.es.html"} {
		raw, _ := os.ReadFile(filepath.Join("testdata", f))
		os.WriteFile(filepath.Join(dir, f), raw, 0o644)
	}
	// es com uma seção a mais e o lang trocado.
	os.WriteFile(filepath.Join(dir, "terms.es.html"),
		[]byte(`<article lang="en"><section id="quem-oferece"></section><section id="trilhas-pagas"></section><section id="extra"></section></article>`), 0o644)
	got := fmt.Sprint(ValidateSet(dir, false))
	for _, want := range []string{`lang="en"`, "ids de seção diferentes"} {
		if !strings.Contains(got, want) {
			t.Errorf("faltou %q em %s", want, got)
		}
	}
	os.Remove(filepath.Join(dir, "privacy.en.html"))
	if got := fmt.Sprint(ValidateSet(dir, false)); !strings.Contains(got, "privacy.en.html") {
		t.Errorf("arquivo faltando não acusado: %s", got)
	}
}

func TestPageMode(t *testing.T) {
	store := newFakeStore(t, PtBR, En, Es)
	cases := map[string]string{
		PtBR: "Versão 2 · Vigente a partir de 1 de outubro de 2026",
		En:   "Version 2 · Effective from October 1, 2026",
		Es:   "Versión 2 · Vigente desde 1 de octubre de 2026",
	}
	for locale, meta := range cases {
		rec := serve(t, Handler(store, Terms, false), "/legal/terms?lang="+locale, nil)
		body := rec.Body.String()
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d", locale, rec.Code)
		}
		if strings.Count(body, "<h1>") != 1 {
			t.Errorf("%s: esperava um h1", locale)
		}
		if strings.Count(body, `class="legal-meta"`) != 1 || !strings.Contains(body, meta) {
			t.Errorf("%s: linha de meta errada ou repetida:\n%s", locale, body)
		}
		if !strings.Contains(body, `<html lang="`+locale+`">`) {
			t.Errorf("%s: html lang errado", locale)
		}
		if !strings.Contains(body, `href="/legal/privacy?lang=`+locale+`"`) {
			t.Errorf("%s: link interno sem a língua", locale)
		}
		if !strings.Contains(body, "prefers-color-scheme:light") || !strings.Contains(body, `class="topbar"`) {
			t.Errorf("%s: página sem tema claro ou sem barra", locale)
		}
	}
}

func TestInternalLinkKeepsFragment(t *testing.T) {
	store := newFakeStore(t, PtBR, En, Es)
	body := serve(t, Handler(store, Privacy, false), "/legal/privacy?lang=es", nil).Body.String()
	if !strings.Contains(body, `href="/legal/terms?lang=es#trilhas-pagas"`) {
		t.Fatalf("âncora perdida:\n%s", body)
	}
}

func TestEmbedMode(t *testing.T) {
	store := newFakeStore(t, PtBR, En, Es)
	body := serve(t, Handler(store, Terms, false), "/legal/terms?embed=1&lang=en", nil).Body.String()
	for _, absent := range []string{"<h1", "legal-meta", "prefers-color-scheme", `class="topbar"`} {
		if strings.Contains(body, absent) {
			t.Errorf("embed não devia ter %q", absent)
		}
	}
	if !strings.Contains(body, "Introduction of the test document.") {
		t.Error("embed perdeu a introdução")
	}
	if !strings.Contains(body, "<title>Test Terms</title>") {
		t.Error("o título continua no <title> para acessibilidade")
	}
}

func TestEmbedOnlyWithOne(t *testing.T) {
	store := newFakeStore(t, PtBR, En, Es)
	body := serve(t, Handler(store, Terms, false), "/legal/terms?embed=true&lang=en", nil).Body.String()
	if !strings.Contains(body, "<h1>") {
		t.Error("só embed=1 liga o modo do app")
	}
}

func TestHighlight(t *testing.T) {
	store := newFakeStore(t, PtBR, En, Es)
	h := Handler(store, Terms, false)

	body := serve(t, h, "/legal/terms?embed=1&lang=es&highlight=trilhas-pagas", nil).Body.String()
	if !strings.Contains(body, `<section id="trilhas-pagas" class="legal-new"><p class="legal-badge">Nuevo en la versión 2</p>`) {
		t.Fatalf("seção não destacada:\n%s", body)
	}
	if strings.Contains(body, `id="quem-oferece" class="legal-new"`) {
		t.Error("destacou seção não pedida")
	}

	body = serve(t, h, "/legal/terms?lang=en&highlight=quem-oferece,trilhas-pagas", nil).Body.String()
	if strings.Count(body, `class="legal-new"`) != 2 {
		t.Error("dois ids, duas seções")
	}

	body = serve(t, h, `/legal/terms?lang=en&highlight=%22%3E%3Cscript%3Ex%3C%2Fscript%3E,nao-existe`, nil).Body.String()
	if strings.Contains(body, `class="legal-new"`) {
		t.Error("id desconhecido destacou alguma coisa")
	}
	if strings.Contains(body, "<script>") || strings.Contains(body, "nao-existe") {
		t.Error("o valor do highlight voltou na saída")
	}
}

func TestPickHighlightsCaps(t *testing.T) {
	known := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
	got := pickHighlights(append(known, "a"), known)
	if len(got) != MaxHighlights {
		t.Fatalf("got %d ids", len(got))
	}
}

func TestDraft(t *testing.T) {
	store := newFakeStore(t, PtBR, En, Es)

	rec := serve(t, Handler(store, Terms, true), "/legal/terms?lang=pt-BR", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("estrito com rascunho: status %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "A CONFIRMAR") {
		t.Error("o 503 vazou o texto")
	}

	rec = serve(t, Handler(store, Terms, false), "/legal/terms?lang=pt-BR", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `class="draft"`) || rec.Header().Get("X-LogN-Legal-Draft") != "1" {
		t.Fatal("fora do estrito o rascunho sai com a faixa")
	}

	// en não tem marcador: sai sem faixa mesmo no estrito.
	rec = serve(t, Handler(store, Terms, true), "/legal/terms?lang=en", nil)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), `class="draft"`) {
		t.Fatal("documento final não leva faixa")
	}
}

func TestHeaders(t *testing.T) {
	store := newFakeStore(t, PtBR, En, Es)
	for _, target := range []string{"/legal/terms?lang=es", "/legal/terms?lang=es&embed=1"} {
		rec := serve(t, Handler(store, Terms, false), target, nil)
		h := rec.Header()
		want := map[string]string{
			"Content-Type":           "text/html; charset=utf-8",
			"Content-Language":       "es",
			"Cache-Control":          "public, max-age=300",
			"Vary":                   "Accept-Language",
			"X-Content-Type-Options": "nosniff",
			"Referrer-Policy":        "no-referrer",
			"X-LogN-Legal-Version":   "2",
			"X-LogN-Legal-Effective": "2026-10-01",
		}
		for k, v := range want {
			if h.Get(k) != v {
				t.Errorf("%s: %s = %q, want %q", target, k, h.Get(k), v)
			}
		}

		style := regexp.MustCompile(`(?s)<style>(.*?)</style>`).FindStringSubmatch(rec.Body.String())
		if style == nil {
			t.Fatal("sem <style>")
		}
		if !strings.Contains(h.Get("Content-Security-Policy"), "'"+hashCSS(style[1])+"'") {
			t.Errorf("%s: hash da CSP não bate com o <style> servido", target)
		}
		if strings.Contains(h.Get("Content-Security-Policy"), "unsafe-inline") {
			t.Error("CSP com unsafe-inline")
		}
	}
}

func TestFallbackToPortuguese(t *testing.T) {
	store := newFakeStore(t, PtBR)
	rec := serve(t, Handler(store, Terms, false), "/legal/terms?lang=es", nil)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Language") != PtBR {
		t.Fatalf("sem espanhol devia cair no português: %d %q", rec.Code, rec.Header().Get("Content-Language"))
	}
}

func TestErrors(t *testing.T) {
	empty := &fakeStore{docs: map[string]Document{}}
	if rec := serve(t, Handler(empty, Terms, false), "/legal/terms", nil); rec.Code != http.StatusNotFound {
		t.Errorf("store vazio: %d", rec.Code)
	}
	broken := &fakeStore{err: errors.New("conexão recusada em 10.0.0.1")}
	rec := serve(t, Handler(broken, Terms, false), "/legal/terms", nil)
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "10.0.0.1") {
		t.Errorf("erro do store: %d %q", rec.Code, rec.Body.String())
	}

	bad := &fakeStore{docs: map[string]Document{"terms/pt-BR": {Kind: Terms, Locale: PtBR, Version: 1, Body: `<article><script>x</script></article>`}}}
	if rec := serve(t, Handler(bad, Terms, false), "/legal/terms", nil); rec.Code != http.StatusInternalServerError {
		t.Errorf("documento inválido: %d", rec.Code)
	}
}
