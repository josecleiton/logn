package email

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io"
	"os"
	"time"

	"gopkg.in/gomail.v2"
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
	Email       string
	Code        string
	CodeSpaced  string
	D1, D2, D3, D4, D5, D6 string
	Purpose     string
	RequestMeta string
}

func newOTPData(email, code, purpose string) OTPData {
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

	meta := fmt.Sprintf("%s · LogN App", time.Now().Format("02 Jan 2006, 15:04"))

	return OTPData{
		Email:       email,
		Code:        code,
		CodeSpaced:  spaced,
		D1: digits[0], D2: digits[1], D3: digits[2],
		D4: digits[3], D5: digits[4], D6: digits[5],
		Purpose:     purpose,
		RequestMeta: meta,
	}
}

func (m *Mailer) SendOTP(toEmail, purpose, code string) error {
	data := newOTPData(toEmail, code, purpose)

	templateName := "otp.html"
	subject := "Seu código de verificação — LogN"

	if purpose == "reset_password" {
		templateName = "reset_password.html"
		subject = "Redefinição de senha — LogN"
	}

	return m.send(toEmail, subject, templateName, data)
}

func (m *Mailer) SendWelcome(toEmail string) error {
	// welcome.html from Claude Design — static, no dynamic fields needed
	// We still render it via template in case future fields are added
	data := struct{ Email string }{Email: toEmail}
	return m.send(toEmail, "Bem-vindo ao LogN!", "welcome.html", data)
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
func NewOTPData(email, code, purpose string) OTPData { return newOTPData(email, code, purpose) }

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
