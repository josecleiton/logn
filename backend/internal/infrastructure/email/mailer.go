package email

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io"
	"os"
	"strings"
	"time"

	"gopkg.in/gomail.v2"

	"github.com/josecleiton/logn/backend/internal/domain"
)

//go:embed templates/*.html templates/assets/*
var templatesFS embed.FS

type Mailer struct {
	templates *template.Template
	host      string
	port      int
	user      string
	pass      string
	from      string
}

func NewMailer() *Mailer {
	tmpl := template.Must(template.ParseFS(templatesFS, "templates/*.html"))

	host := os.Getenv("SMTP_HOST")
	if host == "" {
		host = "localhost"
	}

	portStr := os.Getenv("SMTP_PORT")
	port := 1025
	if portStr != "" {
		fmt.Sscanf(portStr, "%d", &port)
	}

	from := os.Getenv("SMTP_FROM")
	if from == "" {
		from = "LogN <noreply@logn.sh>"
	}

	return &Mailer{
		templates: tmpl,
		host:      host,
		port:      port,
		user:      os.Getenv("SMTP_USER"),
		pass:      os.Getenv("SMTP_PASS"),
		from:      from,
	}
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
func (m *Mailer) SendOTP(toEmail, purpose, code, lang string) error {
	data := newOTPData(toEmail, code, purpose, lang)
	return m.send(toEmail, data.T.Subject, otpTemplate(purpose), data)
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
func (m *Mailer) SendWelcome(toEmail, lang string) error {
	data := NewWelcomeData(lang)
	return m.send(toEmail, data.T.Subject, "welcome.html", data)
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

func (m *Mailer) send(to, subject, templateName string, data interface{}) error {
	var body bytes.Buffer

	if err := m.templates.ExecuteTemplate(&body, templateName, data); err != nil {
		return fmt.Errorf("falha ao renderizar template %s: %w", templateName, err)
	}

	msg := gomail.NewMessage()
	msg.SetHeader("From", m.from)
	msg.SetHeader("To", to)
	msg.SetHeader("Subject", subject)

	// Embed logo via CID
	msg.Embed("logo.jpg", gomail.SetCopyFunc(func(w io.Writer) error {
		fileBytes, err := templatesFS.ReadFile("templates/assets/logo.jpg")
		if err != nil {
			return err
		}
		_, err = w.Write(fileBytes)
		return err
	}))

	msg.SetBody("text/html", body.String())

	dialer := gomail.NewDialer(m.host, m.port, m.user, m.pass)
	if err := dialer.DialAndSend(msg); err != nil {
		return fmt.Errorf("falha ao enviar e-mail smtp: %w", err)
	}
	return nil
}
