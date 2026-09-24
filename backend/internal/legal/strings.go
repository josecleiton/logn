package legal

import (
	"fmt"
	"time"
)

// Os poucos textos que o servidor escreve em volta do documento.
//
// Exceção declarada à regra 6 do AGENTS.md: o catálogo de i18n mora em `i18n/`, fora
// do contexto de build do Docker (`backend/`), e o `go:embed` não alcança. São quatro
// frases por língua, e o documento em si continua vindo inteiro do repositório de
// conteúdo.
type pageStrings struct {
	// meta recebe a versão e a data já formatada.
	meta string
	// badge recebe a versão em que a seção entrou ou mudou.
	badge      string
	draftLabel string
	draft      string
	months     [12]string
	// date monta a data a partir de dia, mês por extenso e ano.
	date func(day int, month string, year int) string
}

var stringsByLocale = map[string]pageStrings{
	PtBR: {
		meta:       "Versão %d · Vigente a partir de %s",
		badge:      "Novo na versão %d",
		draftLabel: "Rascunho",
		draft:      "Rascunho em revisão. Os trechos marcados ainda serão confirmados, e este texto não vale como versão final.",
		months: [12]string{"janeiro", "fevereiro", "março", "abril", "maio", "junho",
			"julho", "agosto", "setembro", "outubro", "novembro", "dezembro"},
		date: func(d int, m string, y int) string { return fmt.Sprintf("%d de %s de %d", d, m, y) },
	},
	En: {
		meta:       "Version %d · Effective from %s",
		badge:      "New in version %d",
		draftLabel: "Draft",
		draft:      "Draft under review. The highlighted passages are still to be confirmed, and this text is not the final version.",
		months: [12]string{"January", "February", "March", "April", "May", "June",
			"July", "August", "September", "October", "November", "December"},
		date: func(d int, m string, y int) string { return fmt.Sprintf("%s %d, %d", m, d, y) },
	},
	Es: {
		meta:       "Versión %d · Vigente desde %s",
		badge:      "Nuevo en la versión %d",
		draftLabel: "Borrador",
		draft:      "Borrador en revisión. Los fragmentos marcados aún se confirmarán, y este texto no es la versión final.",
		months: [12]string{"enero", "febrero", "marzo", "abril", "mayo", "junio",
			"julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"},
		date: func(d int, m string, y int) string { return fmt.Sprintf("%d de %s de %d", d, m, y) },
	},
}

// brazil é o fuso da data de vigência. Fixo, para não depender do tzdata da imagem:
// o Brasil não tem horário de verão desde 2019.
var brazil = time.FixedZone("BRT", -3*60*60)

func textsFor(locale string) pageStrings {
	if s, ok := stringsByLocale[locale]; ok {
		return s
	}
	return stringsByLocale[DefaultLocale]
}

// formatDate escreve a data por extenso na língua pedida.
func formatDate(t time.Time, locale string) string {
	s := textsFor(locale)
	t = t.In(brazil)
	return s.date(t.Day(), s.months[t.Month()-1], t.Year())
}

// EffectiveDate é a data de vigência em ISO 8601, no fuso do Brasil. É o que vai no
// cabeçalho `X-LogN-Legal-Effective`, para o app formatar na língua dele.
func EffectiveDate(t time.Time) string {
	return t.In(brazil).Format("2006-01-02")
}
