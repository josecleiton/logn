//go:build !dev

package httpapi

import (
	"net/http"
	"testing"
)

// O binário de produção não tem documentação nenhuma (ADR 0025).
func TestProductionBuildHasNoAPIDocs(t *testing.T) {
	h := newTestHandler(t)
	for _, path := range []string{"/docs", "/docs/init.js", "/docs/swagger.json"} {
		if rr := serve(h, http.MethodGet, path, ""); rr.Code != http.StatusNotFound {
			t.Errorf("GET %s: %d, want 404", path, rr.Code)
		}
	}
}
