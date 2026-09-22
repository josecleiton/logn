package email

import (
	"regexp"
	"strings"
	"testing"
)

// O e-mail de redefinição trazia os seis dígitos escritos à mão no HTML: mostrava
// "7 3 9 1 6 4" em destaque enquanto o código de verdade era outro. Quem lesse o
// e-mail digitava o número errado e levava "código inválido".
func TestOTPTemplatesShowTheRealCode(t *testing.T) {
	mailer := NewMailer()

	const (
		code      = "482913"
		recipient = "jogador@example.com"
	)

	// Cores hexadecimais têm seis caracteres e às vezes seis dígitos (#111316).
	// Fora do estilo, seis dígitos seguidos só podem ser o código.
	hexColor := regexp.MustCompile(`#[0-9A-Fa-f]{6}\b`)
	sixDigits := regexp.MustCompile(`\d{6}`)

	for _, tc := range []struct {
		template        string
		purpose         string
		namesTheAccount bool
	}{
		{"otp.html", "verify_email", false},
		{"reset_password.html", "reset_password", true},
	} {
		t.Run(tc.template, func(t *testing.T) {
			html, err := mailer.Render(tc.template, NewOTPData(recipient, code, tc.purpose))
			if err != nil {
				t.Fatalf("Failed to render: %v", err)
			}

			for _, found := range sixDigits.FindAllString(hexColor.ReplaceAllString(html, ""), -1) {
				if found != code {
					t.Errorf("%s mostra %q, que não é o código %q", tc.template, found, code)
				}
			}

			// E os dígitos em destaque saem um por célula: os seis têm de estar lá.
			for _, d := range code {
				if !strings.Contains(html, ">"+string(d)+"</td>") {
					t.Errorf("%s não mostra o dígito %q no bloco de destaque", tc.template, string(d))
				}
			}

			// Só a redefinição nomeia a conta ("a senha de fulano@..."), e nomear a
			// conta errada é o que este teste existe para impedir.
			if tc.namesTheAccount && !strings.Contains(html, recipient) {
				t.Errorf("%s não nomeia a conta %q", tc.template, recipient)
			}
		})
	}
}
