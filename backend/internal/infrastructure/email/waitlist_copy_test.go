package email

import (
	"html"
	"reflect"
	"strings"
	"testing"

	"github.com/josecleiton/logn/backend/internal/locale"
)

func TestWaitlistEmailRendersInEveryLanguage(t *testing.T) {
	mailer := NewMailer()
	confirm := "https://api.example.com/api/v1/waitlist/confirm?t=abc.def&x=1"
	leave := "https://api.example.com/api/v1/waitlist/leave?t=abc.ghi"
	for _, lang := range locale.Supported {
		out, err := mailer.Render("waitlist.html", NewWaitlistData(lang, confirm, leave))
		if err != nil {
			t.Fatalf("%s: %v", lang, err)
		}
		if !strings.Contains(out, `<html lang="`+lang+`"`) {
			t.Errorf("%s: o HTML não declara a língua", lang)
		}
		if strings.Contains(out, "<no value>") || strings.Contains(out, "%!") {
			t.Errorf("%s: sobrou campo sem preencher", lang)
		}
		// Os dois links saem inteiros, com o `&` escapado no atributo.
		for _, link := range []string{confirm, leave} {
			if !strings.Contains(html.UnescapeString(out), `href="`+link+`"`) {
				t.Errorf("%s: falta o link %s", lang, link)
			}
		}
	}

	for _, lang := range locale.Supported {
		if got, want := filled(waitlistCopies[lang]), filled(waitlistCopies[locale.Default]); !reflect.DeepEqual(got, want) {
			t.Errorf("%s preenche %v, o %s preenche %v", lang, got, locale.Default, want)
		}
	}
	if NewWaitlistData("fr", confirm, leave).Lang != locale.Default {
		t.Error("língua desconhecida não caiu na padrão")
	}
}

func TestWaitlistHeaders(t *testing.T) {
	leave := "https://api.example.com/api/v1/waitlist/leave?t=abc.ghi"
	h := WaitlistHeaders(leave)
	if got := h["List-Unsubscribe"]; got != "<"+leave+">" {
		t.Errorf("List-Unsubscribe = %q", got)
	}
	// O One-Click volta só com um caminho que não passe pelo desafio da borda.
	if _, ok := h["List-Unsubscribe-Post"]; ok {
		t.Error("List-Unsubscribe-Post sem caminho que chegue ao backend")
	}
}
