package httpapi

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"html/template"
	"log"
	"net/http"

	"github.com/josecleiton/logn/backend/internal/domain"
	"github.com/josecleiton/logn/backend/internal/locale"
)

// A página do botão dos links do e-mail da lista de espera. Uma folha de estilo inline,
// liberada na CSP pelo hash, como as páginas de `/legal`; nada de script.

//go:embed waitlist_page.css
var waitlistPageCSS string

//go:embed waitlist_page.html.tmpl
var waitlistPageSource string

var waitlistPageTemplate = template.Must(template.New("waitlist").Parse(waitlistPageSource))

var waitlistPageCSSHash = func() string {
	sum := sha256.Sum256([]byte(waitlistPageCSS))
	return "sha256-" + base64.StdEncoding.EncodeToString(sum[:])
}()

// waitlistPageCopy é o texto da página numa língua, por ação.
type waitlistPageCopy struct {
	Title, Lead, Button string
}

var waitlistPageCopies = map[string]map[string]waitlistPageCopy{
	locale.PtBR: {
		domain.WaitlistActionConfirm: {
			Title:  "Confirmar inscrição",
			Lead:   "Toque no botão para entrar na lista de espera da versão para iPhone. Mandamos um e-mail só, quando ela sair.",
			Button: "Confirmar",
		},
		domain.WaitlistActionLeave: {
			Title:  "Sair da lista",
			Lead:   "Toque no botão para apagar seu e-mail da lista de espera da versão para iPhone.",
			Button: "Sair da lista",
		},
	},
	locale.En: {
		domain.WaitlistActionConfirm: {
			Title:  "Confirm your spot",
			Lead:   "Tap the button to join the waiting list for the iPhone version. We will send a single e-mail, when it is out.",
			Button: "Confirm",
		},
		domain.WaitlistActionLeave: {
			Title:  "Leave the list",
			Lead:   "Tap the button to delete your e-mail from the waiting list for the iPhone version.",
			Button: "Leave the list",
		},
	},
	locale.Es: {
		domain.WaitlistActionConfirm: {
			Title:  "Confirmar inscripción",
			Lead:   "Toca el botón para entrar en la lista de espera de la versión para iPhone. Enviamos un solo correo, cuando salga.",
			Button: "Confirmar",
		},
		domain.WaitlistActionLeave: {
			Title:  "Salir de la lista",
			Lead:   "Toca el botón para borrar tu correo de la lista de espera de la versión para iPhone.",
			Button: "Salir de la lista",
		},
	},
}

type waitlistPageData struct {
	Lang       string
	CSS        template.CSS
	FormAction string
	T          waitlistPageCopy
}

// writeWaitlistPage responde a página do botão. `formAction` é o caminho da própria
// rota com o token; `landing` entra no `form-action` porque o POST responde com 303 para
// a landing, e o navegador confere o destino do redirect contra ele também.
func writeWaitlistPage(w http.ResponseWriter, lang, action, formAction, landing string) {
	copies, ok := waitlistPageCopies[lang]
	if !ok {
		lang = locale.Default
		copies = waitlistPageCopies[lang]
	}
	var body bytes.Buffer
	err := waitlistPageTemplate.Execute(&body, waitlistPageData{
		Lang: lang, CSS: template.CSS(waitlistPageCSS), FormAction: formAction, T: copies[action],
	})
	if err != nil {
		log.Printf("lista de espera: página não montada: erro=%v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Language", lang)
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	// O token está na URL: nenhum Referer pode levá-lo para fora.
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Robots-Tag", "noindex")
	h.Set("Content-Security-Policy",
		"default-src 'none'; style-src '"+waitlistPageCSSHash+"'; base-uri 'none'; form-action 'self' "+landing+"; frame-ancestors 'none'")
	w.Write(body.Bytes())
}
