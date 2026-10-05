package email

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/josecleiton/logn/backend/internal/domain"
	"github.com/josecleiton/logn/backend/internal/locale"
)

var otpPurposes = []string{domain.OTPPurposeVerifyEmail, domain.OTPPurposeResetPassword}

// filled lista os campos de texto preenchidos de um struct de cópia.
func filled(v any) []string {
	var names []string
	rv := reflect.ValueOf(v)
	for i := 0; i < rv.NumField(); i++ {
		if rv.Field(i).String() != "" {
			names = append(names, rv.Type().Field(i).Name)
		}
	}
	return names
}

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
	// Prazo por extenso ("15 minutos", "15 minutes") e no selo ("15 MIN"). O selo era
	// "15:00", que se lia como horário.
	deadline := regexp.MustCompile(`(\d+) minut[oe]s`)
	badge := regexp.MustCompile(`(\d+) MIN\b`)
	minutes := fmt.Sprint(domain.OTPValidityMinutes())

	for _, lang := range locale.Supported {
		for _, purpose := range otpPurposes {
			tmpl := OTPTemplate(purpose)
			t.Run(lang+"/"+tmpl, func(t *testing.T) {
				html, err := mailer.Render(tmpl, NewOTPData(recipient, code, purpose, lang))
				if err != nil {
					t.Fatalf("Failed to render: %v", err)
				}

				for _, found := range sixDigits.FindAllString(hexColor.ReplaceAllString(html, ""), -1) {
					if found != code {
						t.Errorf("mostra %q, que não é o código %q", found, code)
					}
				}

				// E os dígitos em destaque saem um por célula: os seis têm de estar lá.
				for _, d := range code {
					if !strings.Contains(html, ">"+string(d)+"</td>") {
						t.Errorf("não mostra o dígito %q no bloco de destaque", string(d))
					}
				}

				// O prazo sai da mesma constante que o servidor usa para expirar o
				// código. Estava "10 minutos" à mão enquanto o código valia quinze, e o
				// selo mostrava "10:0" seguido do segundo dígito do código.
				texts := deadline.FindAllStringSubmatch(html, -1)
				badges := badge.FindAllStringSubmatch(html, -1)
				if len(texts) == 0 || len(badges) == 0 {
					t.Errorf("não anuncia o prazo por extenso e no selo (%v, %v)", texts, badges)
				}
				for _, m := range append(texts, badges...) {
					if m[1] != minutes {
						t.Errorf("anuncia %q, mas o código vale %s minutos", m[0], minutes)
					}
				}

				if !strings.Contains(html, `<html lang="`+lang+`"`) {
					t.Errorf("o HTML não declara a língua %q", lang)
				}

				// Só a redefinição nomeia a conta ("a senha de fulano@..."), e nomear a
				// conta errada é o que este teste existe para impedir.
				if purpose == domain.OTPPurposeResetPassword && !strings.Contains(html, recipient) {
					t.Errorf("não nomeia a conta %q", recipient)
				}
			})
		}
	}
}

// Os templates eram escritos em português. Nenhum trecho dele pode sobrar no e-mail
// em inglês.
func TestEnglishOTPEmailHasNoPortuguese(t *testing.T) {
	mailer := NewMailer()
	portuguese := []string{"código", "senha", "você", "solicitado", "Digite", "EXPIRA", "e-mail"}

	for _, purpose := range otpPurposes {
		html, err := mailer.Render(OTPTemplate(purpose), NewOTPData("jogador@example.com", "482913", purpose, locale.En))
		if err != nil {
			t.Fatalf("Failed to render: %v", err)
		}
		for _, word := range portuguese {
			if strings.Contains(html, word) {
				t.Errorf("%s em inglês ainda diz %q", purpose, word)
			}
		}
	}
}

// Tradução com campo vazio vira buraco no e-mail sem nenhum erro. Cada língua tem de
// preencher os mesmos campos que o pt-BR, em que o texto nasce.
func TestOTPCopyCoversEveryLanguage(t *testing.T) {
	for _, lang := range locale.Supported {
		if !reflect.DeepEqual(filled(commonCopies[lang]), filled(commonCopies[locale.Default])) {
			t.Errorf("%s: texto comum com campos diferentes do %s", lang, locale.Default)
		}
		for _, purpose := range otpPurposes {
			got, want := filled(otpCopies[lang][purpose]), filled(otpCopies[locale.Default][purpose])
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s/%s preenche %v, o %s preenche %v", lang, purpose, got, locale.Default, want)
			}
		}
	}
}

// As boas-vindas saem na língua pedida, sem link nenhum: os de logn.sh/start,
// /preferencias e /cancelar davam 404, e o LogN não tem inscrição para cancelar. O
// texto dizia "quatro formatos" quando já eram seis.
func TestWelcomeEmail(t *testing.T) {
	mailer := NewMailer()
	for _, lang := range locale.Supported {
		t.Run(lang, func(t *testing.T) {
			html, err := mailer.Render("welcome.html", NewWelcomeData(lang))
			if err != nil {
				t.Fatalf("Failed to render: %v", err)
			}
			if !strings.Contains(html, `<html lang="`+lang+`"`) {
				t.Errorf("o HTML não declara a língua %q", lang)
			}
			if strings.Contains(html, "href=") {
				t.Errorf("o e-mail de boas-vindas ainda tem link")
			}
			if strings.Contains(html, "{{") || strings.Contains(html, "<no value>") {
				t.Errorf("sobrou campo sem preencher")
			}
		})
	}

	en, _ := mailer.Render("welcome.html", NewWelcomeData(locale.En))
	for _, word := range []string{"Sua conta", "Trilhas", "TRILHAS", "PLACAR", "Você", "formatos"} {
		if strings.Contains(en, word) {
			t.Errorf("boas-vindas em inglês ainda diz %q", word)
		}
	}

	for _, lang := range locale.Supported {
		if got, want := filled(welcomeCopies[lang]), filled(welcomeCopies[locale.Default]); !reflect.DeepEqual(got, want) {
			t.Errorf("%s preenche %v, o %s preenche %v", lang, got, locale.Default, want)
		}
	}
}

// Nenhum e-mail aponta para logn://security: a tela não existe, e o Gmail tira link
// de esquema próprio. Quem não pediu a redefinição não precisa fazer nada.
func TestResetEmailHasNoSecurityLink(t *testing.T) {
	mailer := NewMailer()
	for _, lang := range locale.Supported {
		html, err := mailer.Render(OTPTemplate(domain.OTPPurposeResetPassword),
			NewOTPData("jogador@example.com", "482913", domain.OTPPurposeResetPassword, lang))
		if err != nil {
			t.Fatalf("Failed to render: %v", err)
		}
		if strings.Contains(html, "logn://security") {
			t.Errorf("%s: a redefinição ainda aponta para logn://security", lang)
		}
	}
}

// Língua fora da lista sai na padrão, e não num e-mail vazio.
func TestUnknownLanguageFallsBackToDefault(t *testing.T) {
	data := NewOTPData("jogador@example.com", "482913", domain.OTPPurposeResetPassword, "fr")
	if data.Lang != locale.Default || data.T.Subject != otpCopies[locale.Default][domain.OTPPurposeResetPassword].Subject {
		t.Errorf("língua desconhecida saiu como %q, assunto %q", data.Lang, data.T.Subject)
	}
}
