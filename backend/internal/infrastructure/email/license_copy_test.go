package email

import (
	"reflect"
	"strings"
	"testing"

	"github.com/josecleiton/logn/backend/internal/locale"
)

var licenseKinds = []LicenseNoticeKind{LicenseRevoked, LicenseRevokedAfterReview, LicenseAppealReceived, LicenseUnderReview, LicenseRestored, LicenseAcceptedButRefunded}

func TestLicenseNoticeRendersInEveryLanguage(t *testing.T) {
	mailer := NewMailer()
	for _, lang := range locale.Supported {
		for _, kind := range licenseKinds {
			data, err := NewLicenseData(kind, lang, "Trilha <T>", "account_sharing")
			if err != nil {
				t.Fatal(err)
			}
			html, err := mailer.Render("license.html", data)
			if err != nil {
				t.Fatalf("%s/%d: %v", lang, kind, err)
			}
			if !strings.Contains(html, `<html lang="`+lang+`"`) {
				t.Errorf("%s/%d: o HTML não declara a língua", lang, kind)
			}
			if strings.Contains(html, "{{") || strings.Contains(html, "<no value>") || strings.Contains(html, "%!") {
				t.Errorf("%s/%d: sobrou campo sem preencher", lang, kind)
			}
			// O nome da trilha vem do banco e sai escapado.
			if strings.Contains(html, "<T>") || !strings.Contains(html, "&lt;T&gt;") {
				t.Errorf("%s/%d: o nome da trilha não saiu escapado", lang, kind)
			}
		}
	}

	for _, lang := range locale.Supported {
		if got, want := filled(licenseCopies[lang]), filled(licenseCopies[locale.Default]); !reflect.DeepEqual(got, want) {
			t.Errorf("%s preenche %v, o %s preenche %v", lang, got, locale.Default, want)
		}
	}
}

// A seção 10.5: a revogação diz o motivo e como contestar, e só o compartilhamento de
// conta promete a licença de volta durante a análise.
func TestLicenseNoticeFollowsTheTerms(t *testing.T) {
	sharing, _ := NewLicenseData(LicenseRevoked, locale.PtBR, "Trilha T", "account_sharing")
	redistribution, _ := NewLicenseData(LicenseRevoked, locale.PtBR, "Trilha T", "redistribution")
	for name, d := range map[string]LicenseData{"compartilhamento": sharing, "redistribuição": redistribution} {
		if d.T.Reason == "" || !strings.Contains(d.T.Appeal, "contact@logn.sh") || !strings.Contains(d.T.Appeal, "5 dias") {
			t.Errorf("%s: falta motivo ou contestação: %+v", name, d.T)
		}
		if !strings.Contains(d.T.Consequence, "XP") {
			t.Errorf("%s: não diz o que acontece com o XP", name)
		}
	}
	if !strings.Contains(sharing.T.Appeal, "volta enquanto analisamos") {
		t.Error("compartilhamento não promete a licença durante a análise")
	}
	if strings.Contains(redistribution.T.Appeal, "volta enquanto analisamos") {
		t.Error("redistribuição promete a licença durante a análise")
	}

	if _, err := NewLicenseData(LicenseRevoked, locale.PtBR, "Trilha T", "refund"); err == nil {
		t.Error("motivo da loja virou aviso manual")
	}
	if d, _ := NewLicenseData(LicenseRestored, "fr", "Trilha T", "redistribution"); d.Lang != locale.Default {
		t.Errorf("língua desconhecida saiu em %q", d.Lang)
	}
}
