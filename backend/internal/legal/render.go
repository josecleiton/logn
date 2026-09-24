package legal

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/url"
	"slices"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

//go:embed page.css
var pageCSS string

//go:embed embed.css
var embedCSS string

//go:embed page.html.tmpl
var pageTemplateSource string

var pageTemplate = template.Must(template.New("page").Parse(pageTemplateSource))

// cssHash é o `sha256-…` de cada folha de estilo, para a CSP liberar exatamente o
// `<style>` que a página leva e mais nenhum.
var cssHash = map[bool]string{
	false: hashCSS(pageCSS),
	true:  hashCSS(embedCSS),
}

func hashCSS(css string) string {
	sum := sha256.Sum256([]byte(css))
	return "sha256-" + base64.StdEncoding.EncodeToString(sum[:])
}

// MaxHighlights limita quantas seções um pedido destaca. O app pede as seções que
// mudaram numa versão; mais do que isso é pedido fabricado.
const MaxHighlights = 8

// Options escolhe o modo da página.
type Options struct {
	// Embed troca a página do navegador pela versão de dentro do app.
	Embed bool
	// Highlight lista ids de seção a marcar como novas. Id que o documento não tem é
	// ignorado: o app pode estar pedindo uma seção de versão mais nova.
	Highlight []string
	// Strict recusa documento com marcador de rascunho em vez de servi-lo com a faixa.
	Strict bool
}

// Page é a página pronta, com o que o handler precisa para os cabeçalhos.
type Page struct {
	HTML    []byte
	CSSHash string
	Draft   bool
}

type langLink struct {
	Code, Label, Href string
	Current           bool
}

// endonyms são os nomes das línguas nelas mesmas. Não passam por tradução: quem não lê
// a língua da página precisa achar a sua.
var endonyms = map[string]string{PtBR: "Português", En: "English", Es: "Español"}

// Render monta a página de um documento.
//
// O fragmento é validado de novo aqui, mesmo tendo passado pelo `legalcheck`: é a
// última porta antes do navegador, e o corpo só entra na página depois de relido pelo
// parser e reescrito por ele.
func Render(doc Document, opts Options) (Page, error) {
	ids, err := Validate(doc.Body)
	if err != nil {
		return Page{}, fmt.Errorf("documento %s/%s v%d inválido: %w", doc.Kind, doc.Locale, doc.Version, err)
	}
	draft := ContainsPlaceholder(doc.Body)
	if draft && opts.Strict {
		return Page{}, ErrDraft
	}

	nodes, err := parseFragment(doc.Body)
	if err != nil {
		return Page{}, err
	}
	texts := textsFor(doc.Locale)
	highlight := pickHighlights(opts.Highlight, ids)

	var title string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		for c := n.FirstChild; c != nil; {
			next := c.NextSibling
			if c.Type != html.ElementNode {
				c = next
				continue
			}
			switch {
			case c.DataAtom == atom.P && hasClass(c, "legal-meta"):
				// A versão e a data vêm das colunas do banco, não do texto. O
				// fragmento que ainda traz a linha dele perde a linha aqui.
				n.RemoveChild(c)
			case c.DataAtom == atom.H1:
				if title == "" {
					title = textOf(c)
				}
				if opts.Embed {
					n.RemoveChild(c)
				} else {
					meta := element(atom.P, "class", "legal-meta")
					meta.AppendChild(textNode(fmt.Sprintf(texts.meta, doc.Version, formatDate(doc.EffectiveAt, doc.Locale))))
					n.InsertBefore(meta, next)
				}
			case c.DataAtom == atom.A:
				rewriteInternalLink(c, doc.Locale)
				walk(c)
			case c.DataAtom == atom.Section && slices.Contains(highlight, attr(c, "id")):
				setAttr(c, "class", "legal-new")
				badge := element(atom.P, "class", "legal-badge")
				badge.AppendChild(textNode(fmt.Sprintf(texts.badge, doc.Version)))
				c.InsertBefore(badge, c.FirstChild)
				walk(c)
			default:
				walk(c)
			}
			c = next
		}
	}
	root := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	for _, n := range nodes {
		root.AppendChild(n)
	}
	walk(root)

	var body bytes.Buffer
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		if err := html.Render(&body, c); err != nil {
			return Page{}, err
		}
	}

	css := pageCSS
	if opts.Embed {
		css = embedCSS
	}
	var langs []langLink
	for _, code := range Locales {
		langs = append(langs, langLink{
			Code:    code,
			Label:   endonyms[code],
			Href:    "?lang=" + url.QueryEscape(code),
			Current: code == doc.Locale,
		})
	}

	var out bytes.Buffer
	err = pageTemplate.Execute(&out, map[string]any{
		"Lang":       doc.Locale,
		"Title":      title,
		"Embed":      opts.Embed,
		"CSS":        template.CSS(css),
		"Body":       template.HTML(body.String()), // saída de html.Render sobre fragmento validado
		"Draft":      draft,
		"DraftLabel": texts.draftLabel,
		"DraftText":  texts.draft,
		"Langs":      langs,
	})
	if err != nil {
		return Page{}, err
	}
	return Page{HTML: out.Bytes(), CSSHash: cssHash[opts.Embed], Draft: draft}, nil
}

// pickHighlights fica só com os ids que o documento tem, sem repetição, até o limite.
func pickHighlights(requested, known []string) []string {
	var out []string
	for _, id := range requested {
		if len(out) == MaxHighlights {
			break
		}
		id = strings.TrimSpace(id)
		if slices.Contains(known, id) && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

// rewriteInternalLink leva a língua junto no link para o outro documento. Sem isso,
// quem lê em espanhol cairia no outro documento em português, porque o navegador manda
// o `Accept-Language` dele e não a língua escolhida na página.
func rewriteInternalLink(a *html.Node, locale string) {
	href := attr(a, "href")
	if !internalLink.MatchString(href) {
		return
	}
	path, fragment, _ := strings.Cut(href, "#")
	rewritten := path + "?lang=" + url.QueryEscape(locale)
	if fragment != "" {
		rewritten += "#" + fragment
	}
	setAttr(a, "href", rewritten)
}

func element(a atom.Atom, attrs ...string) *html.Node {
	n := &html.Node{Type: html.ElementNode, Data: a.String(), DataAtom: a}
	for i := 0; i+1 < len(attrs); i += 2 {
		n.Attr = append(n.Attr, html.Attribute{Key: attrs[i], Val: attrs[i+1]})
	}
	return n
}

func textNode(s string) *html.Node { return &html.Node{Type: html.TextNode, Data: s} }

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func setAttr(n *html.Node, key, val string) {
	for i, a := range n.Attr {
		if a.Key == key {
			n.Attr[i].Val = val
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: key, Val: val})
}

func hasClass(n *html.Node, class string) bool {
	return slices.Contains(strings.Fields(attr(n, "class")), class)
}

func textOf(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(b.String())
}
