package legal

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// Handler serve a versão vigente de um tipo de documento.
//
// Contrato:
//   - `?lang=pt-BR|en|es` escolhe a língua; sem ele, vale o `Accept-Language`, e por
//     fim o português;
//   - `?embed=1` devolve o modo do app;
//   - `?highlight=id1,id2` marca seções como novas, nos dois modos.
//
// Com strict, documento com marcador de rascunho responde 503 em vez de sair com a
// faixa de rascunho.
func Handler(store Store, kind Kind, strict bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		locale := Negotiate(r)
		doc, err := store.Latest(r.Context(), kind, locale)
		if errors.Is(err, ErrNotFound) && locale != DefaultLocale {
			// Falta a tradução: a versão que prevalece é a portuguesa.
			doc, err = store.Latest(r.Context(), kind, DefaultLocale)
		}
		if errors.Is(err, ErrNotFound) {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("documento legal não lido: kind=%s locale=%s erro=%v", kind, locale, err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		q := r.URL.Query()
		opts := Options{
			Embed:  q.Get("embed") == "1",
			Strict: strict,
		}
		if h := q.Get("highlight"); h != "" {
			opts.Highlight = strings.Split(h, ",")
		}

		page, err := Render(doc, opts)
		if errors.Is(err, ErrDraft) {
			log.Printf("DOCUMENTO LEGAL EM RASCUNHO NO AR: kind=%s locale=%s v%d — publique o texto final ou defina LEGAL_ALLOW_DRAFT=true",
				kind, doc.Locale, doc.Version)
			http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
			return
		}
		if err != nil {
			log.Printf("documento legal não montado: kind=%s locale=%s erro=%v", kind, doc.Locale, err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		h := w.Header()
		h.Set("Content-Type", "text/html; charset=utf-8")
		h.Set("Content-Language", doc.Locale)
		h.Set("Cache-Control", "public, max-age=300")
		h.Add("Vary", "Accept-Language")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy",
			"default-src 'none'; style-src '"+page.CSSHash+"'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
		h.Set("X-LogN-Legal-Version", strconv.Itoa(doc.Version))
		h.Set("X-LogN-Legal-Effective", EffectiveDate(doc.EffectiveAt))
		if page.Draft {
			h.Set("X-LogN-Legal-Draft", "1")
		}
		w.Write(page.HTML)
	}
}
