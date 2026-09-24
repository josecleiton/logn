package legal

import (
	"regexp"
	"strings"

	"golang.org/x/text/language"
)

// DefaultMinimumAge é a idade mínima onde a tabela não diz outra coisa.
const DefaultMinimumAge = 13

// minimumAgeByCountry lista os países que exigem mais que a idade padrão, por código
// ISO 3166-1 alfa-2. Começa vazia: todos os países da distribuição ficam nos 13 anos.
// Um país entra aqui depois de revisão jurídica, e a mudança vale no próximo deploy,
// sem versão nova do app — a tela pergunta a idade ao servidor.
var minimumAgeByCountry = map[string]int{}

var countryInput = regexp.MustCompile(`^[A-Z]{2,3}$`)

// NormalizeCountry leva o código do país à forma do banco: ISO 3166-1 alfa-2
// (`br` → `BR`, `BRA` → `BR`). O app manda alfa-3 quando o país vem da loja
// (`Storefront.countryCode`) e alfa-2 quando vem da região do iPhone; a conversão fica
// aqui porque o Foundation não a faz.
//
// Código ausente devolve vazio, e não erro: cadastro sem país é aceito e usa a idade
// padrão. ok é falso só para o que não é país, que o cadastro recusa.
func NormalizeCountry(code string) (country string, ok bool) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return "", true
	}
	if !countryInput.MatchString(code) {
		return "", false
	}
	region, err := language.ParseRegion(code)
	if err != nil || !region.IsCountry() {
		return "", false
	}
	return region.String(), true
}

// MinimumAge é a idade mínima para criar conta no país, já normalizado.
func MinimumAge(country string) int {
	if age, ok := minimumAgeByCountry[country]; ok {
		return age
	}
	return DefaultMinimumAge
}
