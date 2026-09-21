package email

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"net/smtp"
)

//go:embed templates/*.html
var templatesFS embed.FS

type Mailer struct {
	templates *template.Template
	host      string
	port      string
	from      string
}

func NewMailer() *Mailer {
	// Parse of all embedded templates
	tmpl := template.Must(template.ParseFS(templatesFS, "templates/*.html"))

	return &Mailer{
		templates: tmpl,
		host:      "localhost", // Use "mailpit" se rodando via docker bridge, mas se o app rodar hosteado usa localhost
		port:      "1025",
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
	
	// Cabeçalhos essenciais para e-mail HTML
	body.Write([]byte(fmt.Sprintf("From: %s\r\n", m.from)))
	body.Write([]byte(fmt.Sprintf("To: %s\r\n", to)))
	body.Write([]byte(fmt.Sprintf("Subject: %s\r\n", subject)))
	body.Write([]byte("MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"))

	if err := m.templates.ExecuteTemplate(&body, templateName, data); err != nil {
		return fmt.Errorf("falha ao renderizar template %s: %w", templateName, err)
	}

	// No Mailpit local, não precisamos de autenticação (PlainAuth).
	// Se for mandar pra AWS SES, precisará configurar smtp.PlainAuth
	addr := fmt.Sprintf("%s:%s", m.host, m.port)
	err := smtp.SendMail(addr, nil, m.from, []string{to}, body.Bytes())
	if err != nil {
		return fmt.Errorf("falha ao enviar e-mail smtp: %w", err)
	}
	return nil
}
