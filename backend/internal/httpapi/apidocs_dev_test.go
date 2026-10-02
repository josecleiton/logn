//go:build dev

package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

// No Cloud Run o binário de desenvolvimento não sobe: a documentação não pode ir junto
// com o serviço de verdade.
func TestDevBuildRefusesCloudRun(t *testing.T) {
	t.Setenv("K_SERVICE", "logn-backend")
	if _, err := New(Deps{}); !errors.Is(err, errDevBuildInProduction) {
		t.Fatalf("New com K_SERVICE: err = %v, want errDevBuildInProduction", err)
	}
}

func TestAPIDocsPageLoadsPinnedSwaggerUI(t *testing.T) {
	rr := serve(newTestHandler(t), http.MethodGet, "/docs", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /docs: %d", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{swaggerUIBase, swaggerUIBundleSRI, swaggerUICSSSRI, `src="/docs/init.js"`} {
		if !strings.Contains(body, want) {
			t.Errorf("a página não traz %q", want)
		}
	}
	csp := rr.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "default-src 'none'") || strings.Contains(csp, "unsafe-inline") {
		t.Errorf("CSP frouxa: %q", csp)
	}
}

func TestAPIDocsServesTheSpec(t *testing.T) {
	rr := serve(newTestHandler(t), http.MethodGet, "/docs/swagger.json", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /docs/swagger.json: %d", rr.Code)
	}
	var spec struct {
		Swagger string         `json:"swagger"`
		Paths   map[string]any `json:"paths"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &spec); err != nil || spec.Swagger == "" || len(spec.Paths) == 0 {
		t.Fatalf("spec inválido: err=%v swagger=%q paths=%d", err, spec.Swagger, len(spec.Paths))
	}
}
