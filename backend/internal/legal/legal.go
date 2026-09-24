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
	"strings"
	"time"

	"github.com/josecleiton/logn/backend/internal/locale"
)

// Kind é o tipo de documento. Só existem os dois do CHECK de `legal_documents`.
type Kind string

const (
	Terms   Kind = "terms"
	Privacy Kind = "privacy"
)

// Locales aceitos, na forma em que aparecem no banco. A negociação mora em
// `internal/locale`, que o conteúdo da trilha usa também.
const (
	PtBR = locale.PtBR
	En   = locale.En
	Es   = locale.Es
)

// DefaultLocale é a língua que prevalece nos próprios documentos. Pedido sem língua
// conhecida cai aqui, e não em inglês: é a versão que vale em caso de divergência.
const DefaultLocale = locale.Default

// Locales lista as línguas servidas, na ordem da troca de língua da página.
var Locales = locale.Supported

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

// Negotiate escolhe a língua do pedido (ver `locale.Negotiate`).
func Negotiate(r *http.Request) string {
	return locale.Negotiate(r)
}
