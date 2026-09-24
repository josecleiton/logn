package locale

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMatch(t *testing.T) {
	for tag, want := range map[string]string{
		"pt": PtBR, "pt-PT": PtBR, "PT_br": PtBR, "en": En, "EN-us": En, "es_MX": Es, " es ": Es,
	} {
		if got, ok := Match(tag); !ok || got != want {
			t.Errorf("Match(%q) = %q, %v; esperava %q", tag, got, ok, want)
		}
	}
	for _, tag := range []string{"", "fr", "de-DE", "*"} {
		if got, ok := Match(tag); ok {
			t.Errorf("Match(%q) = %q; esperava nada", tag, got)
		}
	}
}

// O app manda uma língua só no Accept-Language; o navegador manda a lista com pesos.
// `?lang=` ganha dos dois, e valor desconhecido nele só passa a vez.
func TestNegotiate(t *testing.T) {
	cases := []struct{ query, accept, want string }{
		{"", "es", Es},
		{"", "en-US", En},
		{"", "de;q=0.9,es;q=0.8", Es},
		{"", "fr;q=0.2,en;q=0.9", En},
		{"", "en;q=0", PtBR},
		{"", "%%%;;;q=abc", PtBR},
		{"", "", PtBR},
		{"es", "en", Es},
		{"xx", "en", En},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/nodes?lang="+c.query, nil)
		if c.accept != "" {
			req.Header.Set("Accept-Language", c.accept)
		}
		if got := Negotiate(req); got != c.want {
			t.Errorf("lang=%q accept=%q: %q, esperava %q", c.query, c.accept, got, c.want)
		}
	}
}

func TestSupported(t *testing.T) {
	for _, l := range Supported {
		if !IsSupported(l) {
			t.Errorf("%s está em Supported e IsSupported recusa", l)
		}
	}
	if IsSupported("pt") {
		t.Error("IsSupported recebe a forma do banco; pt sem região não é uma delas")
	}
}
