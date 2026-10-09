package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func serveContent(t *testing.T, userID string, body any, ifNoneMatch string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges", nil)
	if ifNoneMatch != "" {
		req.Header.Set("If-None-Match", ifNoneMatch)
	}
	rec := httptest.NewRecorder()
	writeAccountContentJSON(rec, req, "pt-BR", userID, body)
	return rec
}

// O aparelho que já tem o corpo recebe 304 sem corpo; o que tem outro recebe o corpo.
func TestContentAnswersNotModifiedForTheSameBody(t *testing.T) {
	body := []string{"a", "b"}
	first := serveContent(t, "", body, "")
	etag := first.Header().Get("ETag")
	if first.Code != http.StatusOK || !strings.HasPrefix(etag, `W/"`) || first.Body.Len() == 0 {
		t.Fatalf("primeira resposta: %d etag=%q corpo=%d", first.Code, etag, first.Body.Len())
	}

	again := serveContent(t, "", body, etag)
	if again.Code != http.StatusNotModified || again.Body.Len() != 0 {
		t.Fatalf("mesmo corpo: %d com %d bytes, want 304 vazio", again.Code, again.Body.Len())
	}
	if again.Header().Get("ETag") != etag {
		t.Error("o 304 tem de repetir o ETag")
	}

	// Lista e forma forte também valem na comparação fraca.
	if rec := serveContent(t, "", body, `"outro", `+strings.TrimPrefix(etag, "W/")); rec.Code != http.StatusNotModified {
		t.Fatalf("ETag na lista: %d", rec.Code)
	}

	changed := serveContent(t, "", []string{"a", "c"}, etag)
	if changed.Code != http.StatusOK || changed.Header().Get("ETag") == etag {
		t.Fatalf("corpo mudou e veio %d com o mesmo ETag", changed.Code)
	}
}

// Atrás do gzip, como em produção: o 200 sai comprimido com o ETag, e o 304 sai sem
// corpo e sem `Content-Encoding`.
func TestContentNotModifiedBehindGzip(t *testing.T) {
	body := []string{strings.Repeat("desafio ", 300)}
	handler := withGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeAccountContentJSON(w, r, "pt-BR", "conta-a", body)
	}))
	get := func(ifNoneMatch string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		if ifNoneMatch != "" {
			req.Header.Set("If-None-Match", ifNoneMatch)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	first := get("")
	if first.Code != http.StatusOK || first.Header().Get("Content-Encoding") != "gzip" || first.Header().Get("ETag") == "" {
		t.Fatalf("200: %d encoding=%q etag=%q", first.Code, first.Header().Get("Content-Encoding"), first.Header().Get("ETag"))
	}
	again := get(first.Header().Get("ETag"))
	if again.Code != http.StatusNotModified || again.Body.Len() != 0 || again.Header().Get("Content-Encoding") != "" {
		t.Fatalf("304: %d com %d bytes, encoding=%q", again.Code, again.Body.Len(), again.Header().Get("Content-Encoding"))
	}
}

// A resposta da conta pode ficar no aparelho, nunca num cache no caminho, e é sempre
// conferida: outra conta com outro corpo não recebe o 304 da primeira.
func TestAccountContentIsPrivateAndRevalidated(t *testing.T) {
	mine := serveContent(t, "conta-a", []string{"amostra testada"}, "")
	if cc := mine.Header().Get("Cache-Control"); cc != "private, no-cache" {
		t.Fatalf("Cache-Control = %q", cc)
	}
	// Sem `Authorization` no `Vary`: o token troca a cada refresh, e o cache não casaria.
	vary := strings.Join(mine.Header().Values("Vary"), ",")
	if strings.Contains(vary, "Authorization") || !strings.Contains(vary, "Accept-Language") {
		t.Fatalf("Vary = %q", vary)
	}

	other := serveContent(t, "conta-b", []string{"só a gratuita"}, mine.Header().Get("ETag"))
	if other.Code != http.StatusOK || other.Body.Len() == 0 {
		t.Fatalf("outra conta, outro corpo: %d", other.Code)
	}

	public := serveContent(t, "", []string{"x"}, "")
	if cc := public.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("visitante: Cache-Control = %q", cc)
	}
}
