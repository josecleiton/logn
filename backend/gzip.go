package main

import (
	"compress/gzip"
	"net/http"
	"strings"
)

// Cloud Run entrega a resposta como o servidor escreveu: sem gzip aqui, a trilha
// inteira sai com os 57 KB crus a cada abertura do app, e egress é o que o free tier
// tem de mais curto. Comprimida, fica em 17 KB.
//
// Abaixo de gzipMinBytes a resposta sai como veio — o cabeçalho do gzip custa mais do
// que economiza num "ok" de health check. Por isso o corpo fica num buffer até
// passar do limite ou o handler terminar, e só então se decide.
const gzipMinBytes = 1024

func withGzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Accept-Encoding")
		if !acceptsGzip(r) {
			next.ServeHTTP(w, r)
			return
		}

		gw := &gzipResponseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(gw, r)
		// Sem defer de propósito: se o handler entrar em pânico, o net/http derruba a
		// conexão como sempre fez, em vez de este writer despachar um 200 pela metade.
		gw.finish()
	})
}

func acceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		enc, params, _ := strings.Cut(part, ";")
		if strings.EqualFold(strings.TrimSpace(enc), "gzip") {
			return strings.TrimSpace(params) != "q=0"
		}
	}
	return false
}

type gzipResponseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	buf         []byte
	gz          *gzip.Writer // não-nil: decidiu comprimir
	raw         bool         // true: decidiu não comprimir
}

// WriteHeader só guarda o status: o cabeçalho de verdade sai quando se sabe se a
// resposta vai comprimida, porque Content-Encoding tem de ir junto com ele.
func (g *gzipResponseWriter) WriteHeader(status int) {
	if g.wroteHeader {
		return
	}
	g.wroteHeader = true
	g.status = status
}

func (g *gzipResponseWriter) Write(p []byte) (int, error) {
	if g.gz != nil {
		return g.gz.Write(p)
	}
	if g.raw {
		return g.ResponseWriter.Write(p)
	}

	g.buf = append(g.buf, p...)
	if len(g.buf) < gzipMinBytes {
		return len(p), nil
	}

	h := g.Header()
	if h.Get("Content-Encoding") != "" {
		// O handler já codificou o corpo por conta própria; comprimir de novo corromperia.
		g.raw = true
		return len(p), g.flushRaw()
	}

	// Sem Content-Type declarado, o net/http o adivinharia pelos primeiros bytes — que
	// agora seriam do gzip, não do conteúdo. Adivinha-se antes, pelo corpo original.
	if h.Get("Content-Type") == "" {
		h.Set("Content-Type", http.DetectContentType(g.buf))
	}
	h.Set("Content-Encoding", "gzip")
	h.Del("Content-Length")
	g.ResponseWriter.WriteHeader(g.status)

	g.gz = gzip.NewWriter(g.ResponseWriter)
	_, err := g.gz.Write(g.buf)
	g.buf = nil
	return len(p), err
}

func (g *gzipResponseWriter) flushRaw() error {
	g.ResponseWriter.WriteHeader(g.status)
	var err error
	if len(g.buf) > 0 {
		_, err = g.ResponseWriter.Write(g.buf)
	}
	g.buf = nil
	return err
}

func (g *gzipResponseWriter) finish() {
	switch {
	case g.gz != nil:
		g.gz.Close()
	case !g.raw:
		// O handler terminou abaixo do limite: vai sem compressão.
		g.flushRaw()
	}
}
