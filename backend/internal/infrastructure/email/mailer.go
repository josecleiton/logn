package email

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io"

	"gopkg.in/gomail.v2"
)

//go:embed templates/*.html templates/assets/*
var templatesFS embed.FS

type Mailer struct {
	templates *template.Template
	host      string
	port      int
	from      string
}

func NewMailer() *Mailer {
	tmpl := template.Must(template.ParseFS(templatesFS, "templates/*.html"))

	return &Mailer{
		templates: tmpl,
		host:      "localhost", // Use "mailpit" no docker-compose
		port:      1025,
		from:      "LogN <noreply@logn.sh>",
	}
}

func (m *Mailer) SendOTP(toEmail, purpose, code string) error {
	subjectStr := "Código de Verificação"
	if purpose == "reset_password" {
		subjectStr = "Recuperação de Senha"
	}

	data := struct {
		Subject string
		Code    string
	}{
		Subject: subjectStr,
		Code:    code,
	}

	return m.send(toEmail, subjectStr, "otp.html", data)
}

func (m *Mailer) SendWelcome(toEmail string) error {
	data := struct {
		Email string
	}{
		Email: toEmail,
	}

	return m.send(toEmail, "Bem-vindo ao LogN!", "welcome.html", data)
}

func (m *Mailer) send(to, subject, templateName string, data interface{}) error {
	var body bytes.Buffer

	if err := m.templates.ExecuteTemplate(&body, templateName, data); err != nil {
		return fmt.Errorf("falha ao renderizar template %s: %w", templateName, err)
	}

	msg := gomail.NewMessage()
	msg.SetHeader("From", m.from)
	msg.SetHeader("To", to)
	msg.SetHeader("Subject", subject)

	// Anexando a imagem inline via CID
	msg.Embed("logo.jpg", gomail.SetCopyFunc(func(w io.Writer) error {
		fileBytes, err := templatesFS.ReadFile("templates/assets/logo.jpg")
		if err != nil {
			return err
		}
		_, err = w.Write(fileBytes)
		return err
	}))

	msg.SetBody("text/html", body.String())

	dialer := gomail.Dialer{Host: m.host, Port: m.port}
	if err := dialer.DialAndSend(msg); err != nil {
		return fmt.Errorf("falha ao enviar e-mail smtp: %w", err)
	}
	return nil
}
