package httpapi

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/josecleiton/logn/backend/internal/domain"
	"github.com/josecleiton/logn/backend/internal/locale"
)

// Lista de espera do iPhone (ADR 0022). Quem chama é o navegador, não o app: o
// formulário da landing posta aqui sem JS, e os links do e-mail abrem aqui. Por isso a
// resposta é sempre um 303 para uma página da landing, nunca `writeError` — não há
// ninguém lendo JSON do outro lado.
//
// Os links do e-mail não mudam nada no GET. Filtro de e-mail corporativo abre todo link
// que recebe, e um GET que confirmasse inscreveria quem não pediu, e um que tirasse da
// lista tiraria quem pediu. O GET mostra uma página com um botão, e o botão faz o POST.

// WaitlistConfig liga a lista de espera. Nula, as rotas não existem (404).
type WaitlistConfig struct {
	// A landing, para onde vão os 303 (`https://logn.sh`).
	LandingOrigin string
	// O domínio público da API, para os links do e-mail. Nunca a URL `.run.app`, que
	// recusa pedido fora da Cloudflare (ADR 0012).
	APIOrigin string
}

var originPattern = regexp.MustCompile(`^https://[a-z0-9-]+(\.[a-z0-9-]+)+$`)

// WaitlistConfigFromEnv lê `WAITLIST_LANDING_ORIGIN` e `WAITLIST_API_ORIGIN`. As duas
// vazias desligam a lista; uma só, ou uma fora do formato, é erro de configuração, e o
// servidor não sobe com ele.
func WaitlistConfigFromEnv(getenv func(string) string) (*WaitlistConfig, error) {
	landing, api := getenv("WAITLIST_LANDING_ORIGIN"), getenv("WAITLIST_API_ORIGIN")
	if landing == "" && api == "" {
		return nil, nil
	}
	for name, v := range map[string]string{"WAITLIST_LANDING_ORIGIN": landing, "WAITLIST_API_ORIGIN": api} {
		if !originPattern.MatchString(v) {
			return nil, fmt.Errorf("%s tem de ser https://domínio, sem caminho: %q", name, v)
		}
	}
	if regexp.MustCompile(`\.run\.app$`).MatchString(api) {
		return nil, errors.New("WAITLIST_API_ORIGIN não pode ser a URL .run.app")
	}
	return &WaitlistConfig{LandingOrigin: landing, APIOrigin: api}, nil
}

// WaitlistMailer manda o e-mail de confirmação. É o Mailer; os testes trocam.
type WaitlistMailer interface {
	SendWaitlistConfirmation(toEmail, lang, confirmURL, leaveURL string) error
}

// O teto do corpo do formulário: e-mail, língua e a isca cabem em poucas centenas de bytes.
const waitlistBodyLimit = 4 << 10

// As páginas da landing que recebem os 303 (`landing/src/waitlist.html`).
const (
	waitlistThanks    = "thanks"
	waitlistConfirmed = "confirmed"
	waitlistLeft      = "left"
	waitlistError     = "error"
)

// landingPath é o prefixo da língua na landing: pt-BR na raiz.
var landingPath = map[string]string{locale.PtBR: "", locale.En: "en/", locale.Es: "es/"}

func (c *WaitlistConfig) landing(lang, state string) string {
	return c.LandingOrigin + "/" + landingPath[lang] + "waitlist/" + state + "/"
}

func (c *WaitlistConfig) link(action, id string) string {
	return c.APIOrigin + "/api/v1/waitlist/" + action + "?t=" + url.QueryEscape(domain.WaitlistToken(id, action))
}

func (s *Server) waitlistRedirect(w http.ResponseWriter, r *http.Request, lang, state string) {
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, s.waitlist.landing(lang, state), http.StatusSeeOther)
}

// fromLanding diz se o formulário veio da landing. Sem isto, qualquer página na
// internet posta o formulário pelo navegador de quem a visita, um IP diferente por
// visitante, e o limite por IP não vê nada.
//
// A landing sai com `Referrer-Policy: no-referrer`, e com ela o navegador manda
// `Origin: null` no POST do formulário. Por isso vale também o `Sec-Fetch-Site`, que o
// navegador escreve e página nenhuma muda: `same-site` é um subdomínio do mesmo domínio,
// e a landing e a API estão nele. Navegador antigo, sem nenhum dos dois, fica de fora.
func (c *WaitlistConfig) fromLanding(r *http.Request) bool {
	if r.Header.Get("Origin") == c.LandingOrigin {
		return true
	}
	switch r.Header.Get("Sec-Fetch-Site") {
	case "same-site", "same-origin":
		return true
	}
	return false
}

// emailLike acha endereços dentro de uma mensagem de erro.
var emailLike = regexp.MustCompile(`[^\s<>"'(),;:]+@[^\s<>"'(),;:]+`)

// redactEmails é o erro pronto para o log, sem endereço de e-mail. O SMTP costuma
// repetir o destinatário na recusa (`550 <x@y>: Recipient address rejected`), e o log
// não guarda e-mail (política, seção 9).
func redactEmails(err error) string {
	return emailLike.ReplaceAllString(err.Error(), "<e-mail>")
}

// formLocale é a língua do campo `locale`, de lista fechada; fora dela, a padrão.
func formLocale(v string) string {
	if locale.IsSupported(v) {
		return v
	}
	return locale.Default
}

// registerWaitlistRoutes pendura as rotas, se a lista estiver ligada.
func (s *Server) registerWaitlistRoutes(mux *http.ServeMux) {
	if s.waitlist == nil || s.waitlistMailer == nil {
		return
	}
	// Dez inscrições por hora por IP: sobra para uma casa inteira atrás do mesmo NAT, e
	// o reenvio por endereço é travado no banco (WaitlistResendCooldown).
	join := newRateLimiter(10, time.Hour)
	links := newRateLimiter(60, time.Minute)
	mux.HandleFunc("POST /api/v1/waitlist", limitBody(waitlistBodyLimit, s.joinWaitlistHandler(join)))
	for _, action := range []string{domain.WaitlistActionConfirm, domain.WaitlistActionLeave} {
		mux.HandleFunc("GET /api/v1/waitlist/"+action, s.waitlistPageHandler(links, action))
		mux.HandleFunc("POST /api/v1/waitlist/"+action, limitBody(waitlistBodyLimit, s.waitlistActionHandler(links, action)))
	}
}

// joinWaitlistHandler inscreve um e-mail na lista de espera do iPhone (ADR 0022).
//
//	@Summary		Entra na lista de espera
//	@Description	Formulário da landing: `Origin` tem de ser a landing, ou `Sec-Fetch-Site` same-site. Todo desfecho, inclusive erro e limite, é 303 para uma página da landing.
//	@Tags			waitlist
//	@Accept			x-www-form-urlencoded
//	@Param			email	formData	string	true	"E-mail"
//	@Param			locale	formData	string	false	"Língua"	Enums(pt-BR, en, es)
//	@Param			website	formData	string	false	"Honeypot; tem de vir vazio"
//	@Success		303		"redireciona para thanks ou error"
//	@Router			/api/v1/waitlist [post]
func (s *Server) joinWaitlistHandler(limiter *rateLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			s.waitlistRedirect(w, r, locale.Default, waitlistError)
			return
		}
		lang := formLocale(r.PostForm.Get("locale"))
		if !s.waitlist.fromLanding(r) {
			log.Printf("lista de espera: pedido de fora da landing recusado: origin=%.64q sec-fetch-site=%.16q",
				r.Header.Get("Origin"), r.Header.Get("Sec-Fetch-Site"))
			s.waitlistRedirect(w, r, lang, waitlistError)
			return
		}
		if !limiter.allowIP(r) {
			s.waitlistRedirect(w, r, lang, waitlistError)
			return
		}
		// A isca preenchida é robô. Ele recebe o mesmo "confira seu e-mail" de todo
		// mundo, para não aprender qual campo o denunciou, e nada é gravado.
		if r.PostForm.Get("website") != "" {
			log.Printf("lista de espera: isca preenchida, pedido ignorado")
			s.waitlistRedirect(w, r, lang, waitlistThanks)
			return
		}
		addr, err := domain.NormalizeEmail(r.PostForm.Get("email"))
		if err != nil {
			s.waitlistRedirect(w, r, lang, waitlistError)
			return
		}

		_, _, err = s.repo.JoinWaitlist(r.Context(), addr, lang)
		if errors.Is(err, domain.ErrWaitlistBusy) {
			log.Printf("lista de espera: teto de %d envios por hora atingido", domain.WaitlistHourlySendCap)
			s.waitlistRedirect(w, r, lang, waitlistError)
			return
		}
		if err != nil {
			log.Printf("lista de espera: inscrição não gravada: erro=%v", err)
			s.waitlistRedirect(w, r, lang, waitlistError)
			return
		}
		// A confirmação entrou na caixa de saída com a inscrição, e vai para a fila quando
		// o pedido termina (ADR 0026). Pôr na fila custa algumas dezenas de milissegundos
		// que a inscrição repetida não gasta: o tempo de resposta pode dizer, a quem medir,
		// que um endereço é novo na lista. Antes o envio saía numa goroutine para não dizer
		// isso; a troca foi aceita na ADR, porque a goroutine podia nunca rodar.
		//
		// Novo, pendente ou já confirmado: a mesma página.
		s.waitlistRedirect(w, r, lang, waitlistThanks)
	}
}

// waitlistPageHandler mostra a página do botão de um link do e-mail. Link que não vale
// vai para a página de erro da landing.
//
//	@Summary	Página do botão de um link do e-mail
//	@Tags		waitlist
//	@Produce	html
//	@Param		t	query		string	true	"Token assinado do link"
//	@Success	200	{string}	string	"página com o botão"
//	@Success	303	"link inválido ou já confirmado"
//	@Failure	500	{string}	string	"Internal error"
//	@Router		/api/v1/waitlist/confirm [get]
//	@Router		/api/v1/waitlist/leave [get]
func (s *Server) waitlistPageHandler(limiter *rateLimiter, action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !limiter.allowIP(r) {
			s.waitlistRedirect(w, r, locale.Default, waitlistError)
			return
		}
		token := r.URL.Query().Get("t")
		id, ok := domain.ParseWaitlistToken(token, action)
		if !ok {
			s.waitlistRedirect(w, r, locale.Default, waitlistError)
			return
		}
		entry, err := s.repo.GetWaitlistEntry(r.Context(), id)
		if err != nil {
			if !errors.Is(err, domain.ErrWaitlistNotFound) {
				log.Printf("lista de espera: inscrição não lida: id=%s erro=%v", id, err)
			}
			s.waitlistRedirect(w, r, locale.Default, waitlistError)
			return
		}
		// Confirmada não precisa de botão: o link de confirmação aberto de novo vai
		// direto para a página de confirmada.
		if action == domain.WaitlistActionConfirm && entry.Confirmed {
			s.waitlistRedirect(w, r, entry.Locale, waitlistConfirmed)
			return
		}
		writeWaitlistPage(w, entry.Locale, action, "/api/v1/waitlist/"+action+"?t="+url.QueryEscape(token), s.waitlist.LandingOrigin)
	}
}

// waitlistActionHandler faz o que o botão pede.
//
//	@Summary	Confirma ou sai da lista de espera
//	@Tags		waitlist
//	@Param		t	query	string	true	"Token assinado do link"
//	@Success	303	"redireciona para confirmed, left ou error"
//	@Router		/api/v1/waitlist/confirm [post]
//	@Router		/api/v1/waitlist/leave [post]
func (s *Server) waitlistActionHandler(limiter *rateLimiter, action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !limiter.allowIP(r) {
			s.waitlistRedirect(w, r, locale.Default, waitlistError)
			return
		}
		id, ok := domain.ParseWaitlistToken(r.URL.Query().Get("t"), action)
		if !ok {
			s.waitlistRedirect(w, r, locale.Default, waitlistError)
			return
		}

		var lang, state string
		var err error
		switch action {
		case domain.WaitlistActionConfirm:
			lang, err = s.repo.ConfirmWaitlist(r.Context(), id)
			state = waitlistConfirmed
		case domain.WaitlistActionLeave:
			lang, err = s.repo.LeaveWaitlist(r.Context(), id)
			state = waitlistLeft
		}
		if errors.Is(err, domain.ErrWaitlistNotFound) {
			// Sair de novo, com a linha já apagada, é sair: a pessoa está fora da lista.
			if action == domain.WaitlistActionLeave {
				s.waitlistRedirect(w, r, locale.Default, waitlistLeft)
				return
			}
			s.waitlistRedirect(w, r, locale.Default, waitlistError)
			return
		}
		if err != nil {
			log.Printf("lista de espera: %s não gravado: id=%s erro=%v", action, id, err)
			s.waitlistRedirect(w, r, locale.Default, waitlistError)
			return
		}
		log.Printf("lista de espera: %s ok: id=%s", action, id)
		s.waitlistRedirect(w, r, lang, state)
	}
}
