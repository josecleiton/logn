package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func serveGzip(t *testing.T, handler http.HandlerFunc, acceptEncoding string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if acceptEncoding != "" {
		req.Header.Set("Accept-Encoding", acceptEncoding)
	}
	rec := httptest.NewRecorder()
	withGzip(handler).ServeHTTP(rec, req)
	return rec
}

func gunzip(t *testing.T, body []byte) string {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("corpo não é gzip válido: %v", err)
	}
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("descomprimindo: %v", err)
	}
	return string(out)
}

// Escreve em pedaços pequenos, como o json.Encoder faz, para passar pelo buffer.
func bigJSON(status int) (http.HandlerFunc, string) {
	payload := `[` + strings.Repeat(`{"id":"ch_101","template_type":"DRY_RUN"},`, 100) + `{}]`
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		for i := 0; i < len(payload); i += 100 {
			w.Write([]byte(payload[i:min(i+100, len(payload))]))
		}
	}, payload
}

func TestGzipComprimeRespostaGrandeQuandoPedido(t *testing.T) {
	handler, payload := bigJSON(http.StatusOK)
	rec := serveGzip(t, handler, "gzip, deflate, br")

	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, quero gzip", got)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, o do handler tem de sobreviver", got)
	}
	if rec.Body.Len() >= len(payload) {
		t.Errorf("comprimido tem %d bytes, original %d", rec.Body.Len(), len(payload))
	}
	if got := gunzip(t, rec.Body.Bytes()); got != payload {
		t.Errorf("descomprimido difere do original")
	}
}

func TestGzipPreservaStatusDeErro(t *testing.T) {
	handler, _ := bigJSON(http.StatusInternalServerError)
	rec := serveGzip(t, handler, "gzip")

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, quero 500", rec.Code)
	}
}

func TestGzipNaoComprimeSemAcceptEncoding(t *testing.T) {
	handler, payload := bigJSON(http.StatusOK)
	rec := serveGzip(t, handler, "")

	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q sem o cliente pedir", got)
	}
	if rec.Body.String() != payload {
		t.Errorf("corpo alterado sem compressão")
	}
}

func TestGzipRespeitaRecusaExplicita(t *testing.T) {
	handler, _ := bigJSON(http.StatusOK)
	rec := serveGzip(t, handler, "gzip;q=0, identity")

	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q com gzip recusado", got)
	}
}

func TestGzipDeixaRespostaPequenaCrua(t *testing.T) {
	rec := serveGzip(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Database not ready"))
	}, "gzip")

	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q numa resposta de 18 bytes", got)
	}
	if rec.Code != http.StatusServiceUnavailable || rec.Body.String() != "Database not ready" {
		t.Errorf("resposta = %d %q", rec.Code, rec.Body.String())
	}
}

func TestGzipNaoRecomprimeCorpoJaCodificado(t *testing.T) {
	body := strings.Repeat("x", 2*gzipMinBytes)
	rec := serveGzip(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "br")
		w.Write([]byte(body))
	}, "gzip")

	if got := rec.Header().Get("Content-Encoding"); got != "br" {
		t.Fatalf("Content-Encoding = %q, quero o br do handler", got)
	}
	if rec.Body.String() != body {
		t.Errorf("corpo já codificado foi alterado")
	}
}
