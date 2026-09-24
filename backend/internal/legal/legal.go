// Package legal monta as páginas de termos de uso e política de privacidade.
//
// O texto não mora aqui. Ele vive no repositório de conteúdo, chega ao banco por
// migração (`legal_documents`) e entra neste pacote como um fragmento HTML — um
// `<article>` com seções numeradas. Daqui sai uma página só, em dois modos:
//
//   - a página do navegador, com título, versão, data de vigência e tema claro ou
//     escuro. É a URL que vai para a loja;
//   - o modo embed (`?embed=1`), com o CSS do app e sem título nem data, porque a
//     tela nativa já mostra os dois no cabeçalho.
//
// O pacote não conhece o banco: quem busca o documento é um `Store`.
package legal

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Kind é o tipo de documento. Só existem os dois do CHECK de `legal_documents`.
type Kind string

const (
	Terms   Kind = "terms"
	Privacy Kind = "privacy"
)

// Locales aceitos, na forma em que aparecem no banco.
const (
	PtBR = "pt-BR"
	En   = "en"
	Es   = "es"
)

// DefaultLocale é a língua que prevalece nos próprios documentos. Pedido sem língua
// conhecida cai aqui, e não em inglês: é a versão que vale em caso de divergência.
const DefaultLocale = PtBR

// Locales lista as línguas servidas, na ordem da troca de língua da página.
var Locales = []string{PtBR, En, Es}

// Document é uma versão publicada de um documento numa língua.
type Document struct {
	Kind        Kind
	Locale      string
	Version     int
	EffectiveAt time.Time
	Body        string
}

// Store entrega a versão vigente de um documento numa língua.
type Store interface {
	Latest(ctx context.Context, kind Kind, locale string) (Document, error)
}

var (
	// ErrNotFound: não há documento publicado para esse tipo e língua.
	ErrNotFound = errors.New("legal document not found")
	// ErrDraft: o documento ainda tem marcador de rascunho e o servidor está em modo
	// estrito.
	ErrDraft = errors.New("legal document is still a draft")
)

// placeholders são os marcadores que o rascunho usa em cada língua. Nenhum pode
// chegar a quem lê o documento como texto final.
var placeholders = []string{"A CONFIRMAR", "TO CONFIRM", "POR CONFIRMAR"}

// ContainsPlaceholder diz se o corpo ainda tem marcador de rascunho.
func ContainsPlaceholder(body string) bool {
	for _, p := range placeholders {
		if strings.Contains(body, p) {
			return true
		}
	}
	return false
}

// Negotiate escolhe a língua do pedido: `?lang=` primeiro, depois `Accept-Language`,
// e por fim a língua padrão. Valor desconhecido em `?lang=` é ignorado, não é erro —
// a negociação só segue para a próxima fonte.
func Negotiate(r *http.Request) string {
	if l, ok := matchLocale(r.URL.Query().Get("lang")); ok {
		return l
	}
	for _, tag := range acceptLanguage(r.Header.Get("Accept-Language")) {
		if l, ok := matchLocale(tag); ok {
			return l
		}
	}
	return DefaultLocale
}

// matchLocale leva uma etiqueta de língua (`pt`, `pt-PT`, `EN-us`, `es_MX`) a um dos
// locales servidos. Português de qualquer região vira `pt-BR`, que é o único que
// existe.
func matchLocale(tag string) (string, bool) {
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
		pos int
	}
	var tags []weighted
	for i, part := range strings.Split(header, ",") {
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
		tags = append(tags, weighted{tag, q, i})
	}
	sort.SliceStable(tags, func(a, b int) bool { return tags[a].q > tags[b].q })
	out := make([]string, len(tags))
	for i, t := range tags {
		out[i] = t.tag
	}
	return out
}
