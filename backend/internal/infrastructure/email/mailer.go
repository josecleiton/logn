package email

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"net"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	gomail "github.com/wneessen/go-mail"

	"github.com/josecleiton/logn/backend/internal/domain"
)

//go:embed templates/*.html
var templatesFS embed.FS

// smtpsPort é a porta do TLS implícito: a conexão já nasce cifrada. Em qualquer outra,
// a cifra vem do STARTTLS (ADR 0027).
const smtpsPort = 465

type Mailer struct {
	templates *template.Template
	host      string
	port      int
	user      string
	pass      string
	from      string
	// requireTLS recusa servidor sem STARTTLS fora da 465. Em produção é sempre ligado:
	// opcional, quem está no meio tira a oferta e lê o OTP em claro. No desenvolvimento
	// fica desligado, porque o Mailpit não fala TLS.
	requireTLS bool
	// implicitTLS cifra a conexão no dial, sem STARTTLS. É a porta ser a 465; fica num
	// campo para o teste ligar num servidor de porta qualquer.
	implicitTLS bool
	// rootCAs são as raízes que validam o certificado do servidor. Nulo, as do sistema;
	// só o teste troca.
	rootCAs *x509.CertPool
	// msgIDDomain é o domínio do Message-ID, o mesmo do remetente.
	msgIDDomain string
}

// NewMailer é o Mailer do desenvolvimento, com os padrões de NewMailerFromEnv: Mailpit
// em localhost:1025, sem TLS e sem login. Serve aos testes que só renderizam.
func NewMailer() *Mailer {
	m, err := NewMailerFromEnv(func(string) string { return "" }, false)
	if err != nil {
		panic(err)
	}
	return m
}

// NewMailerFromEnv monta o envio de SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASS e
// SMTP_FROM. Em produção, faltar qualquer um dos quatro primeiros é erro: o padrão é o
// Mailpit, e o OTP sumia sem alarme num deploy que perdesse uma variável.
func NewMailerFromEnv(getenv func(string) string, production bool) (*Mailer, error) {
	if production {
		var missing []string
		for _, k := range []string{"SMTP_HOST", "SMTP_PORT", "SMTP_USER", "SMTP_PASS"} {
			if getenv(k) == "" {
				missing = append(missing, k)
			}
		}
		if len(missing) > 0 {
			return nil, fmt.Errorf("faltam %s", strings.Join(missing, ", "))
		}
	}

	host := getenv("SMTP_HOST")
	if host == "" {
		host = "localhost"
	}

	port := 1025
	if s := getenv("SMTP_PORT"); s != "" {
		p, err := strconv.Atoi(s)
		if err != nil || p < 1 || p > 65535 {
			return nil, fmt.Errorf("SMTP_PORT inválida: %q", s)
		}
		port = p
	}

	from := getenv("SMTP_FROM")
	if from == "" {
		from = "LogN <noreply@logn.sh>"
	}
	addr, err := mail.ParseAddress(from)
	if err != nil {
		return nil, fmt.Errorf("SMTP_FROM inválido: %w", err)
	}

	return &Mailer{
		templates:   template.Must(template.ParseFS(templatesFS, "templates/*.html")),
		host:        host,
		port:        port,
		user:        getenv("SMTP_USER"),
		pass:        getenv("SMTP_PASS"),
		from:        from,
		requireTLS:  production,
		implicitTLS: port == smtpsPort,
		msgIDDomain: addr.Address[strings.LastIndex(addr.Address, "@")+1:],
	}, nil
}

type OTPData struct {
	Email                  string
	Code                   string
	CodeSpaced             string
	D1, D2, D3, D4, D5, D6 string
	Purpose                string
	// Lang é a língua do e-mail, já resolvida para uma das servidas.
	Lang string
	// T é o texto do e-mail na língua, com código, prazo e data já preenchidos.
	T OTPText
}

// OTPText é o texto pronto de um e-mail de código. O prazo sai de domain.OTPValidity:
// estava escrito "10 minutos" à mão enquanto o código valia quinze.
type OTPText struct {
	Subject, Preheader, Eyebrow, Heading, Lead, LeadAfter, Expires, Button string
	NotYouLabel, NotYou                                                    string
	RequestedAt, Footer                                                    string
}

func newOTPData(email, code, purpose, lang string) OTPData {
	spaced := ""
	for i, c := range code {
		if i > 0 {
			spaced += " "
		}
		spaced += string(c)
	}

	digits := make([]string, 6)
	for i := 0; i < 6 && i < len(code); i++ {
		digits[i] = string(code[i])
	}

	c, common, lang := copyFor(lang, purpose)
	minutes := domain.OTPValidityMinutes()
	// Só formata o campo que tem verbo; os outros passam como estão.
	withMinutes := func(s string) string {
		if !strings.Contains(s, "%d") {
			return s
		}
		return fmt.Sprintf(s, minutes)
	}
	requested := fmt.Sprintf("%s · LogN App", time.Now().Format(common.DateLayout))

	return OTPData{
		Email:      email,
		Code:       code,
		CodeSpaced: spaced,
		D1:         digits[0], D2: digits[1], D3: digits[2],
		D4: digits[3], D5: digits[4], D6: digits[5],
		Purpose: purpose,
		Lang:    lang,
		T: OTPText{
			Subject:     c.Subject,
			Preheader:   fmt.Sprintf(c.Preheader, spaced, minutes),
			Eyebrow:     c.Eyebrow,
			Heading:     c.Heading,
			Lead:        withMinutes(c.Lead),
			LeadAfter:   withMinutes(c.LeadAfter),
			Expires:     fmt.Sprintf(common.Expires, minutes),
			Button:      c.Button,
			NotYouLabel: c.NotYouLabel,
			NotYou:      c.NotYou,
			RequestedAt: fmt.Sprintf(common.RequestedAt, requested),
			Footer:      common.Footer,
		},
	}
}

// otpTemplate é o HTML de cada propósito de código.
func otpTemplate(purpose string) string {
	if purpose == domain.OTPPurposeResetPassword {
		return "reset_password.html"
	}
	return "otp.html"
}

// SendOTP manda o código na língua pedida (ver locale.Negotiate); língua desconhecida
// sai em locale.Default.
func (m *Mailer) SendOTP(ctx context.Context, toEmail, purpose, code, lang string) error {
	data := newOTPData(toEmail, code, purpose, lang)
	return m.send(ctx, toEmail, data.T.Subject, otpTemplate(purpose), data)
}

// WelcomeData é o que o e-mail de boas-vindas recebe.
type WelcomeData struct {
	Lang string
	T    welcomeCopy
}

// NewWelcomeData monta os dados do e-mail de boas-vindas, para teste e para o envio.
func NewWelcomeData(lang string) WelcomeData {
	c, lang := welcomeFor(lang)
	return WelcomeData{Lang: lang, T: c}
}

// SendWelcome manda as boas-vindas na língua pedida, depois do cadastro.
func (m *Mailer) SendWelcome(ctx context.Context, toEmail, lang string) error {
	data := NewWelcomeData(lang)
	return m.send(ctx, toEmail, data.T.Subject, "welcome.html", data)
}

// Render devolve o HTML final de um template. Existe para o teste poder olhar o que
// o destinatário veria: o template de redefinição trazia seis dígitos escritos à mão,
// então o e-mail mostrava em destaque um código que nunca ia funcionar.
func (m *Mailer) Render(templateName string, data interface{}) (string, error) {
	var body bytes.Buffer
	if err := m.templates.ExecuteTemplate(&body, templateName, data); err != nil {
		return "", fmt.Errorf("falha ao renderizar template %s: %w", templateName, err)
	}
	return body.String(), nil
}

// NewOTPData monta os dados de um e-mail de código, para teste e para o envio.
func NewOTPData(email, code, purpose, lang string) OTPData {
	return newOTPData(email, code, purpose, lang)
}

// OTPTemplate diz qual HTML leva o código de um propósito, para o teste renderizar o
// mesmo que o envio.
func OTPTemplate(purpose string) string { return otpTemplate(purpose) }

func (m *Mailer) send(ctx context.Context, to, subject, templateName string, data interface{}) error {
	return m.sendWithHeaders(ctx, to, subject, templateName, data, nil)
}

// sendWithHeaders é o send com cabeçalhos a mais. Quem chama escreve valores fixos ou
// URLs montadas pelo servidor, nunca texto do pedido.
//
// O erro sai sem endereço de e-mail: o SMTP repete o destinatário na recusa, e a
// biblioteca também, e quem chama grava o erro no log e em `last_error`.
func (m *Mailer) sendWithHeaders(ctx context.Context, to, subject, templateName string, data interface{}, headers map[string]string) error {
	var body bytes.Buffer

	if err := m.templates.ExecuteTemplate(&body, templateName, data); err != nil {
		return fmt.Errorf("falha ao renderizar template %s: %s", templateName, RedactAddresses(err.Error()))
	}

	// Sem User-Agent nem X-Mailer: diriam a quem recebe qual biblioteca, e qual versão,
	// manda o e-mail.
	msg := gomail.NewMsg(gomail.WithNoDefaultUserAgent())
	if err := msg.From(m.from); err != nil {
		return fmt.Errorf("remetente inválido: %s", RedactAddresses(err.Error()))
	}
	if err := msg.To(to); err != nil {
		return fmt.Errorf("destinatário inválido: %s", RedactAddresses(err.Error()))
	}
	msg.Subject(subject)
	msg.SetDate()
	msg.SetMessageIDWithValue(m.messageID())
	for k, v := range headers {
		msg.SetGenHeader(gomail.Header(k), v)
	}
	msg.SetBodyString(gomail.TypeTextHTML, body.String())

	if err := m.deliver(ctx, msg); err != nil {
		redacted := RedactAddresses(err.Error())
		// O fim do ctx segue na cadeia: quem chama distingue prazo de recusa.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("falha ao enviar e-mail smtp: %w: %s", ctxErr, redacted)
		}
		return fmt.Errorf("falha ao enviar e-mail smtp: %s", redacted)
	}
	return nil
}

// deliver manda a mensagem e desiste quando o ctx acaba. A biblioteca só olha o ctx
// para conectar; depois disso, cada passo tem o próprio prazo, que ela renova. Por isso a
// conexão é aberta aqui, e fechada por baixo quando o ctx acaba: o envio parado num
// `DATA` sem resposta cai na hora, em vez de segurar a linha do outbox.
func (m *Mailer) deliver(ctx context.Context, msg *gomail.Msg) error {
	var (
		mu     sync.Mutex
		conn   net.Conn
		closed bool
	)
	tlsConfig := &tls.Config{ServerName: m.host, MinVersion: tls.VersionTLS12, RootCAs: m.rootCAs}
	dial := func(dialCtx context.Context, network, addr string) (net.Conn, error) {
		c, err := (&net.Dialer{}).DialContext(dialCtx, network, addr)
		if err != nil {
			return nil, err
		}
		mu.Lock()
		// O ctx pode ter acabado antes de a conexão ficar à vista do AfterFunc.
		if closed {
			mu.Unlock()
			c.Close()
			return nil, ctx.Err()
		}
		conn = c
		mu.Unlock()
		if !m.implicitTLS {
			return c, nil
		}
		tc := tls.Client(c, tlsConfig)
		if err := tc.HandshakeContext(dialCtx); err != nil {
			c.Close()
			return nil, err
		}
		return tc, nil
	}
	stop := context.AfterFunc(ctx, func() {
		mu.Lock()
		defer mu.Unlock()
		closed = true
		if conn != nil {
			conn.Close()
		}
	})
	defer stop()

	// HELO fixo, como era com o gomail: o padrão da biblioteca é o hostname, que no Cloud
	// Run é o da instância.
	opts := []gomail.Option{
		gomail.WithPort(m.port), gomail.WithDialContextFunc(dial), gomail.WithHELO("localhost"),
		gomail.WithTLSConfig(tlsConfig),
	}
	switch {
	case m.implicitTLS:
		// A cifra já vem do dial; sem isto a biblioteca tentaria STARTTLS por cima.
		opts = append(opts, gomail.WithSSL())
	case m.requireTLS:
		opts = append(opts, gomail.WithTLSPolicy(gomail.TLSMandatory))
	default:
		opts = append(opts, gomail.WithTLSPolicy(gomail.TLSOpportunistic))
	}
	if m.user != "" {
		// PLAIN fixo, sem a escolha automática da biblioteca. Ela recusa PLAIN em conexão
		// sem TLS, a não ser com localhost; em produção a conexão já é cifrada antes do
		// login pela política acima, e esta é a segunda trava.
		opts = append(opts,
			gomail.WithSMTPAuth(gomail.SMTPAuthPlain),
			gomail.WithUsername(m.user),
			gomail.WithPassword(m.pass),
		)
	}
	client, err := gomail.NewClient(m.host, opts...)
	if err != nil {
		return err
	}
	return client.DialAndSendWithContext(ctx, msg)
}

// messageID é um Message-ID aleatório no domínio do remetente. O padrão da biblioteca
// usa o hostname, que no Cloud Run é o da instância.
func (m *Mailer) messageID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b) + "@" + m.msgIDDomain
}

// emailLike acha endereços dentro de uma mensagem de erro.
var emailLike = regexp.MustCompile(`[^\s<>"'(),;:]+@[^\s<>"'(),;:]+`)

// RedactAddresses troca todo endereço de e-mail de um texto por `<e-mail>`. O log não
// guarda e-mail (política, seção 9).
func RedactAddresses(s string) string {
	return emailLike.ReplaceAllString(s, "<e-mail>")
}
