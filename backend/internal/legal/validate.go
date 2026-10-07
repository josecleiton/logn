package legal

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// allowedTags é tudo o que um documento pode usar. O texto é nosso, mas vem de outro
// repositório por migração: uma tag fora da lista é engano de edição, e barrar aqui
// custa menos do que descobrir um `<script>` na página que vai para a loja.
var allowedTags = map[atom.Atom]bool{
	atom.Article: true, atom.Header: true, atom.Section: true,
	atom.H1: true, atom.H2: true, atom.H3: true, atom.P: true,
	atom.Ul: true, atom.Ol: true, atom.Li: true,
	atom.Strong: true, atom.Em: true, atom.A: true, atom.Br: true, atom.Mark: true,
	atom.Table: true, atom.Thead: true, atom.Tbody: true, atom.Tr: true, atom.Th: true, atom.Td: true,
}

// allowedAttrs diz que atributo cabe em que tag.
var allowedAttrs = map[string]map[atom.Atom]bool{
	"class": {atom.Article: true, atom.P: true},
	"lang":  {atom.Article: true},
	"id":    {atom.Section: true},
	"href":  {atom.A: true},
}

var (
	sectionIDPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	// internalLink é o único caminho relativo aceito: o outro documento.
	internalLink = regexp.MustCompile(`^/legal/(terms|privacy)(#[a-z0-9-]+)?$`)
)

// Validate confere a estrutura de um fragmento e devolve os ids das seções, na ordem
// em que aparecem.
func Validate(body string) ([]string, error) {
	nodes, err := parseFragment(body)
	if err != nil {
		return nil, err
	}
	var ids []string
	var walk func(*html.Node) error
	walk = func(n *html.Node) error {
		switch n.Type {
		case html.ElementNode:
			if !allowedTags[n.DataAtom] {
				return fmt.Errorf("tag not allowed: <%s>", n.Data)
			}
			for _, a := range n.Attr {
				if !allowedAttrs[a.Key][n.DataAtom] {
					return fmt.Errorf("attribute not allowed: %s on <%s>", a.Key, n.Data)
				}
				if a.Key == "href" && !validHref(a.Val) {
					return fmt.Errorf("link not allowed: %q", a.Val)
				}
				if a.Key == "id" {
					if !sectionIDPattern.MatchString(a.Val) {
						return fmt.Errorf("section id out of pattern: %q", a.Val)
					}
					if slices.Contains(ids, a.Val) {
						return fmt.Errorf("repeated section id: %q", a.Val)
					}
					ids = append(ids, a.Val)
				}
			}
		case html.CommentNode, html.DoctypeNode:
			return fmt.Errorf("comment or doctype in the fragment")
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if err := walk(c); err != nil {
				return err
			}
		}
		return nil
	}
	for _, n := range nodes {
		if err := walk(n); err != nil {
			return nil, err
		}
	}
	return ids, nil
}

func validHref(v string) bool {
	return strings.HasPrefix(v, "mailto:") || strings.HasPrefix(v, "https://") || internalLink.MatchString(v)
}

// articleLang devolve o `lang` do primeiro `<article>` do fragmento.
func articleLang(body string) string {
	nodes, err := parseFragment(body)
	if err != nil {
		return ""
	}
	for _, n := range nodes {
		if n.Type == html.ElementNode && n.DataAtom == atom.Article {
			for _, a := range n.Attr {
				if a.Key == "lang" {
					return a.Val
				}
			}
		}
	}
	return ""
}

// ValidateSet confere os seis arquivos de uma pasta — `<kind>.<locale>.html` — como um
// conjunto: todos presentes, cada um válido, o `lang` batendo com o nome, e os mesmos
// ids de seção nas três línguas. É o que permite destacar uma seção pelo id em qualquer
// língua.
//
// Com release, recusa também os marcadores de rascunho e o `<mark>` que os envolve.
func ValidateSet(dir string, release bool) []error {
	var errs []error
	for _, kind := range []Kind{Terms, Privacy} {
		var reference []string
		for _, locale := range Locales {
			name := fmt.Sprintf("%s.%s.html", kind, locale)
			raw, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", name, err))
				continue
			}
			body := string(raw)
			ids, err := Validate(body)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", name, err))
				continue
			}
			if lang := articleLang(body); lang != locale {
				errs = append(errs, fmt.Errorf("%s: <article lang=%q>, expected %q", name, lang, locale))
			}
			if release && (ContainsPlaceholder(body) || strings.Contains(body, "<mark")) {
				errs = append(errs, fmt.Errorf("%s: still has a draft marker", name))
			}
			if reference == nil {
				reference = ids
			} else if !slices.Equal(sorted(reference), sorted(ids)) {
				errs = append(errs, fmt.Errorf("%s: section ids differ from %s.%s.html", name, kind, Locales[0]))
			}
		}
	}
	return errs
}

func sorted(s []string) []string {
	c := slices.Clone(s)
	slices.Sort(c)
	return c
}

// parseFragment lê o fragmento como conteúdo de `<body>`.
func parseFragment(body string) ([]*html.Node, error) {
	ctx := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(body), ctx)
	if err != nil {
		return nil, fmt.Errorf("unreadable fragment: %w", err)
	}
	return nodes, nil
}
