// Package locale escolhe a língua de uma resposta: documentos legais e conteúdo da
// trilha saem em pt-BR, en ou es, e o pedido diz qual.
package locale

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// As línguas servidas, na forma em que aparecem no banco.
const (
	PtBR = "pt-BR"
	En   = "en"
	Es   = "es"
)

// Default é a língua de quem pede sem língua conhecida. Português, e não inglês: é a
// versão que prevalece nos documentos e a língua em que o conteúdo nasce.
const Default = PtBR

// Supported lista as línguas servidas, na ordem da troca de língua das páginas.
var Supported = []string{PtBR, En, Es}

// IsSupported diz se a língua, já na forma do banco, é uma das servidas.
func IsSupported(l string) bool {
	for _, s := range Supported {
		if s == l {
			return true
		}
	}
	return false
}

// Negotiate escolhe a língua do pedido: `?lang=` primeiro, depois `Accept-Language`, e
// por fim a padrão. Valor desconhecido em `?lang=` é ignorado, não é erro — a
// negociação só segue para a próxima fonte.
//
// O app manda no `Accept-Language` a língua da própria interface, uma só; a lista com
// pesos é o caso do navegador.
func Negotiate(r *http.Request) string {
	if l, ok := Match(r.URL.Query().Get("lang")); ok {
		return l
	}
	for _, tag := range acceptLanguage(r.Header.Get("Accept-Language")) {
		if l, ok := Match(tag); ok {
			return l
		}
	}
	return Default
}

// Match leva uma etiqueta de língua (`pt`, `pt-PT`, `EN-us`, `es_MX`) a uma das servidas.
// Português de qualquer região vira `pt-BR`, que é o único que existe.
func Match(tag string) (string, bool) {
	tag = strings.ToLower(strings.TrimSpace(tag))
	if tag == "" {
		return "", false
	}
	base, _, _ := strings.Cut(strings.ReplaceAll(tag, "_", "-"), "-")
	switch base {
	case "pt":
		return PtBR, true
	case "en":
		return En, true
	case "es":
		return Es, true
	}
	return "", false
}

// acceptLanguage devolve as etiquetas do cabeçalho em ordem de preferência. Entrada
// malformada vira lista vazia, e a negociação cai na língua padrão.
func acceptLanguage(header string) []string {
	type weighted struct {
		tag string
		q   float64
	}
	var tags []weighted
	for _, part := range strings.Split(header, ",") {
		tag, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		tag = strings.TrimSpace(tag)
		if tag == "" || tag == "*" {
			continue
		}
		q := 1.0
		if v, ok := strings.CutPrefix(strings.TrimSpace(params), "q="); ok {
			parsed, err := strconv.ParseFloat(v, 64)
			if err != nil {
				continue
			}
			q = parsed
		}
		if q <= 0 {
			continue
		}
		tags = append(tags, weighted{tag, q})
	}
	sort.SliceStable(tags, func(a, b int) bool { return tags[a].q > tags[b].q })
	out := make([]string, len(tags))
	for i, t := range tags {
		out[i] = t.tag
	}
	return out
}
