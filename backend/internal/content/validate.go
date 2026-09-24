// Package content confere os arquivos da trilha antes de virarem migração.
//
// A fonte do conteúdo é a pasta trilha/ do repositório de conteúdo (ver
// exporta_trilha.py e gen_conteudo.py lá): um arquivo de estrutura por nó e por
// desafio, e um de texto por língua. O banco confere a forma do payload com CHECKs, mas
// só na hora da migração, em produção; aqui o erro aparece antes, com o nome do arquivo.
package content

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/josecleiton/logn/backend/internal/locale"
)

// Finding é um achado da conferência. Pendente é trabalho que ainda falta fazer — a
// tradução, a reescrita do código em inglês — e só vira erro com --release; o resto é
// erro sempre.
type Finding struct {
	Where   string
	Msg     string
	Pending bool
}

func (f Finding) String() string { return f.Where + ": " + f.Msg }

type node struct {
	ID            string   `json:"id"`
	Row           int      `json:"row"`
	Col           int      `json:"col"`
	RequiredXP    int      `json:"required_xp"`
	Prerequisites []string `json:"prerequisites"`
	Topic         string   `json:"topic"`
}

type nodeText struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type challenge struct {
	ID           string `json:"id"`
	NodeID       string `json:"node_id"`
	TemplateType string `json:"template_type"`
	PositionIdx  int    `json:"position_idx"`
	Origin       string `json:"origin"`
	// De onde o desafio partiu, para quem escreve. Fica no repositório de conteúdo e
	// não vai para o banco.
	Fonte   string          `json:"fonte"`
	Payload json.RawMessage `json:"payload"`
}

type payload struct {
	Content struct {
		CodeLines      []string `json:"code_lines"`
		Options        []string `json:"options"`
		CorrectOptions []string `json:"correct_options"`
		WatchVariables []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"watch_variables"`
	} `json:"content"`
	Validation struct {
		Type           string  `json:"type"`
		CorrectLine    *int    `json:"correct_line"`
		ExpectedString *string `json:"expected_string"`
		TraceCells     *int    `json:"trace_cells"`
	} `json:"validation"`
}

type challengeText struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Explanation string `json:"explanation"`
	WatchNote   string `json:"watch_note"`
}

// originCard é o texto do cartão de origem numa língua: quem escreveu o desafio,
// quando não foi escrito para o LogN. `origens.json` traz o pt-BR (o id nasce ali);
// `origens.<locale>.json` traz as outras línguas, indexadas pelo mesmo id.
type originCard struct {
	Name string `json:"name"`
	Role string `json:"role"`
	Body string `json:"body"`
}

type originEntry struct {
	ID string `json:"id"`
	originCard
}

var (
	topicRe = regexp.MustCompile(`^[a-z][a-z_]*$`)
	slugRe  = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	identRe = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)
	// Texto que o app mostra cru: marcação vira caractere solto na tela. Colchete depois
	// de identificador é índice citado do código (`v[i]`) e passa; solto é marcador de
	// rascunho ou link de markdown.
	markupRe = regexp.MustCompile("[*`]|(?:^|[^A-Za-z0-9_\\])])\\[")
)

var templates = map[string]bool{
	"SPOT_THE_BUG": true, "DRY_RUN": true, "FILL_IN_THE_BLANK": true,
	"COMPLEXITY_MATCH": true, "TAG_THE_PATTERN": true,
}

// Palavras em português tiradas dos identificadores do código da trilha antes da
// reescrita. Identificador que tenha uma delas (inteira, ou como parte de camelCase)
// ainda não foi reescrito em inglês.
var portugueseWords = map[string]bool{
	"aceitas": true, "achados": true, "achar": true, "adicionar": true, "alvo": true,
	"antes": true, "aresta": true, "arestas": true, "atual": true, "balanceado": true,
	"busca": true, "conjunto": true, "contar": true, "contem": true, "correto": true,
	"custo": true, "divisores": true, "em": true, "fatores": true, "fila": true,
	"fim": true, "formas": true, "frente": true, "gerar": true, "grupo": true,
	"guardar": true, "imprimir": true, "infinito": true, "iniciar": true, "inicio": true,
	"lento": true, "livre": true, "maior": true, "mais": true, "mdc": true,
	"melhor": true, "mesmo": true, "mostrar": true, "novo": true, "ordem": true,
	"origem": true, "passos": true, "perfeito": true, "peso": true, "pilha": true,
	"pontos": true, "primo": true, "prox": true, "rapido": true, "remover": true,
	"reserva": true, "sobe": true, "soma": true, "subida": true, "unir": true,
	"usado": true, "valor": true, "visitado": true, "viz": true, "volta": true,
}

// O valor de watch que o app mostra antes de a variável existir.
const watchPlaceholder = "—"

type checker struct {
	release  bool
	findings []Finding
}

func (c *checker) fail(where, format string, args ...any) {
	c.findings = append(c.findings, Finding{Where: where, Msg: fmt.Sprintf(format, args...)})
}

func (c *checker) pending(where, format string, args ...any) {
	c.findings = append(c.findings, Finding{Where: where, Msg: fmt.Sprintf(format, args...), Pending: true})
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// Validate confere a trilha em root (a pasta com trilha/ e glossario/). Devolve todos
// os achados; quem chama decide o que é erro pelo campo Pending e pelo modo.
func Validate(root string) []Finding {
	c := &checker{}
	trilha := filepath.Join(root, "trilha")

	var nodes []node
	if err := readJSON(filepath.Join(trilha, "nos.json"), &nodes); err != nil {
		c.fail("trilha/nos.json", "%v", err)
		return c.findings
	}
	nodeIDs := c.checkNodes(nodes)
	c.checkNodeTexts(trilha, nodeIDs)

	glossary := map[string]map[string]string{}
	if err := readJSON(filepath.Join(root, "glossario", "tags.json"), &glossary); err != nil && !os.IsNotExist(err) {
		c.fail("glossario/tags.json", "%v", err)
	}
	c.checkGlossary(glossary)

	c.checkChallenges(trilha, nodeIDs, glossary)
	return c.findings
}

func (c *checker) checkNodes(nodes []node) map[string]bool {
	ids := map[string]bool{}
	cells := map[[2]int]string{}
	for _, n := range nodes {
		where := "trilha/nos.json " + n.ID
		if ids[n.ID] {
			c.fail(where, "id repetido")
		}
		ids[n.ID] = true
		if other, ok := cells[[2]int{n.Row, n.Col}]; ok {
			c.fail(where, "mesma posição (row %d, col %d) que %s", n.Row, n.Col, other)
		}
		cells[[2]int{n.Row, n.Col}] = n.ID
		switch {
		case n.Topic == "":
			c.pending(where, "sem topic: o app adivinha a cor pelo nome, e erra fora do português")
		case !topicRe.MatchString(n.Topic):
			c.fail(where, "topic %q fora de ^[a-z][a-z_]*$", n.Topic)
		}
	}
	for _, n := range nodes {
		for _, p := range n.Prerequisites {
			if !ids[p] {
				c.fail("trilha/nos.json "+n.ID, "pré-requisito %s não existe", p)
			}
		}
	}
	return ids
}

func (c *checker) checkNodeTexts(trilha string, nodeIDs map[string]bool) {
	for _, l := range locale.Supported {
		name := "nos." + l + ".json"
		texts := map[string]nodeText{}
		err := readJSON(filepath.Join(trilha, name), &texts)
		if os.IsNotExist(err) {
			c.missingLocale("trilha/"+name, l, "os nomes dos nós")
			continue
		}
		if err != nil {
			c.fail("trilha/"+name, "%v", err)
			continue
		}
		for id := range nodeIDs {
			t, ok := texts[id]
			switch {
			case !ok:
				c.missingLocale("trilha/"+name+" "+id, l, "o nó")
			case strings.TrimSpace(t.Name) == "":
				c.fail("trilha/"+name+" "+id, "nome vazio")
			default:
				c.checkText("trilha/"+name+" "+id, t.Name, t.Description)
			}
		}
		for id := range texts {
			if !nodeIDs[id] {
				c.fail("trilha/"+name, "texto para o nó %s, que não existe em nos.json", id)
			}
		}
	}
}

func (c *checker) checkGlossary(glossary map[string]map[string]string) {
	seen := map[string]map[string]string{}
	for _, slug := range sortedKeys(glossary) {
		where := "glossario/tags.json " + slug
		if !slugRe.MatchString(slug) {
			c.fail(where, "slug fora de ^[a-z][a-z0-9_]*$")
		}
		for _, l := range locale.Supported {
			label := strings.TrimSpace(glossary[slug][l])
			if label == "" {
				c.missingLocale(where, l, "o rótulo")
				continue
			}
			if seen[l] == nil {
				seen[l] = map[string]string{}
			}
			if other, ok := seen[l][strings.ToLower(label)]; ok {
				c.fail(where, "rótulo %q em %s repete o de %s", label, l, other)
			}
			seen[l][strings.ToLower(label)] = slug
		}
	}
}

func (c *checker) checkChallenges(trilha string, nodeIDs map[string]bool, glossary map[string]map[string]string) {
	dir := filepath.Join(trilha, "desafios")
	entries, err := os.ReadDir(dir)
	if err != nil {
		c.fail("trilha/desafios", "%v", err)
		return
	}

	ids := map[string]bool{}
	positions := map[string]map[int]string{}
	// originsUsed lembra, para cada desafio com origin não vazio, de qual origem —
	// para a checagem do cartão nomear o desafio e não só o id da origem.
	originsUsed := map[string]string{}
	var texts []string
	for _, e := range entries {
		base, ok := strings.CutSuffix(e.Name(), ".json")
		if !ok {
			continue
		}
		if strings.Contains(base, ".") {
			texts = append(texts, base)
			continue
		}
		where := "trilha/desafios/" + e.Name()
		var ch challenge
		if err := readJSON(filepath.Join(dir, e.Name()), &ch); err != nil {
			c.fail(where, "%v", err)
			continue
		}
		if ch.ID != base {
			c.fail(where, "o id dentro (%q) não bate com o nome do arquivo", ch.ID)
		}
		ids[base] = true
		if !nodeIDs[ch.NodeID] {
			c.fail(where, "nó %s não existe em nos.json", ch.NodeID)
		}
		if positions[ch.NodeID] == nil {
			positions[ch.NodeID] = map[int]string{}
		}
		if other, ok := positions[ch.NodeID][ch.PositionIdx]; ok {
			c.fail(where, "position_idx %d repete o de %s no mesmo nó", ch.PositionIdx, other)
		}
		positions[ch.NodeID][ch.PositionIdx] = base
		if ch.PositionIdx < 1 {
			c.fail(where, "position_idx começa em 1")
		}
		if strings.TrimSpace(ch.Origin) != "" {
			originsUsed[base] = ch.Origin
		}
		p := c.checkPayload(where, ch, glossary)

		for _, l := range locale.Supported {
			c.checkChallengeText(dir, base, l, p)
		}
	}

	for _, t := range texts {
		id, l, _ := strings.Cut(t, ".")
		if !ids[id] {
			c.fail("trilha/desafios/"+t+".json", "texto de um desafio que não existe")
		}
		if !locale.IsSupported(l) {
			c.fail("trilha/desafios/"+t+".json", "língua %q não é servida", l)
		}
	}

	c.checkOrigins(trilha, originsUsed)
}

// checkOrigins confere que toda origem usada por um desafio tem cartão nas três
// línguas. `origens.json` traz o pt-BR e é onde o id nasce; `origens.<locale>.json`
// traz as outras, no mesmo formato de `nos.<locale>.json`. O erro nomeia o desafio, não
// só a origem, porque é o desafio que a pessoa está editando quando lê o achado.
func (c *checker) checkOrigins(trilha string, originsUsed map[string]string) {
	if len(originsUsed) == 0 {
		return
	}

	cardsByLocale := map[string]map[string]originCard{}

	var base []originEntry
	if err := readJSON(filepath.Join(trilha, "origens.json"), &base); err != nil {
		c.fail("trilha/origens.json", "%v", err)
		return
	}
	baseCards := map[string]originCard{}
	for _, o := range base {
		baseCards[o.ID] = o.originCard
	}
	cardsByLocale[locale.PtBR] = baseCards

	for _, l := range locale.Supported {
		if l == locale.PtBR {
			continue
		}
		name := "origens." + l + ".json"
		texts := map[string]originCard{}
		err := readJSON(filepath.Join(trilha, name), &texts)
		if os.IsNotExist(err) {
			cardsByLocale[l] = map[string]originCard{}
			continue
		}
		if err != nil {
			c.fail("trilha/"+name, "%v", err)
			continue
		}
		cardsByLocale[l] = texts
	}

	for _, challengeID := range sortedKeys(originsUsed) {
		origin := originsUsed[challengeID]
		where := "trilha/desafios/" + challengeID + ".json"
		for _, l := range locale.Supported {
			card, ok := cardsByLocale[l][origin]
			switch {
			case !ok:
				c.missingLocale(where, l, "o cartão de origem "+origin)
			case strings.TrimSpace(card.Name) == "" || strings.TrimSpace(card.Role) == "" || strings.TrimSpace(card.Body) == "":
				c.fail(where, "cartão de origem %s em %s com campo vazio", origin, l)
			default:
				c.checkText(where, card.Name, card.Role, card.Body)
			}
		}
	}
}

// checkPayload confere as mesmas regras das CHECKs do banco, mais as da reescrita.
func (c *checker) checkPayload(where string, ch challenge, glossary map[string]map[string]string) *payload {
	if !templates[ch.TemplateType] {
		c.fail(where, "template %q desconhecido", ch.TemplateType)
		return nil
	}

	var raw map[string]map[string]json.RawMessage
	if err := json.Unmarshal(ch.Payload, &raw); err != nil || raw["content"] == nil || raw["validation"] == nil {
		c.fail(where, "payload sem content ou validation")
		return nil
	}
	for _, k := range []string{"title", "description", "watch_note"} {
		if _, ok := raw["content"][k]; ok {
			c.fail(where, "content.%s é texto e mora no arquivo da língua", k)
		}
	}
	if _, ok := raw["validation"]["explanation"]; ok {
		c.fail(where, "validation.explanation é texto e mora no arquivo da língua")
	}

	var p payload
	if err := json.Unmarshal(ch.Payload, &p); err != nil {
		c.fail(where, "payload fora da forma: %v", err)
		return nil
	}
	ct, v := p.Content, p.Validation
	code := strings.Join(ct.CodeLines, "\n")

	switch ch.TemplateType {
	case "SPOT_THE_BUG":
		if v.CorrectLine == nil || *v.CorrectLine < 1 || *v.CorrectLine > len(ct.CodeLines) {
			c.fail(where, "correct_line fora das %d linhas", len(ct.CodeLines))
		}
	case "DRY_RUN":
		if v.Type != "OUTPUT_MATCH" {
			c.fail(where, "validation.type de DRY_RUN é OUTPUT_MATCH")
		}
		if v.ExpectedString == nil || strings.TrimSpace(*v.ExpectedString) == "" {
			c.fail(where, "expected_string vazio")
		} else if strings.ContainsFunc(strings.TrimSpace(*v.ExpectedString), unicode.IsSpace) {
			c.fail(where, "expected_string %q tem mais de um token", *v.ExpectedString)
		}
		if v.TraceCells == nil || *v.TraceCells < 1 || *v.TraceCells > 4 {
			c.fail(where, "trace_cells entre 1 e 4")
		}
		if len(ct.WatchVariables) == 0 {
			c.fail(where, "DRY_RUN sem watch_variables")
		}
		for _, w := range ct.WatchVariables {
			if !regexp.MustCompile(`\b` + regexp.QuoteMeta(w.Name) + `\b`).MatchString(code) {
				c.fail(where, "a variável de watch %q não aparece no código", w.Name)
			}
			if !isASCII(w.Name) || (w.Value != watchPlaceholder && !isASCII(w.Value)) {
				c.pending(where, "watch %q = %q fora de ASCII", w.Name, w.Value)
			}
		}
	case "FILL_IN_THE_BLANK":
		if len(ct.Options) < 2 {
			c.fail(where, "FILL com menos de duas opções")
		}
		if v.ExpectedString == nil || !contains(ct.Options, *v.ExpectedString) {
			c.fail(where, "expected_string fora das opções")
		}
		if !strings.Contains(code, "_____") {
			c.fail(where, "código sem a lacuna _____")
		}
		for _, o := range ct.Options {
			if !isASCII(o) {
				c.pending(where, "opção %q fora de ASCII", o)
			}
		}
	case "COMPLEXITY_MATCH", "TAG_THE_PATTERN":
		if len(ct.Options) < 2 || len(ct.CorrectOptions) == 0 {
			c.fail(where, "precisa de ao menos duas opções e uma certa")
		}
		for _, o := range ct.CorrectOptions {
			if !contains(ct.Options, o) {
				c.fail(where, "resposta certa %q fora das opções", o)
			}
		}
		if ch.TemplateType == "COMPLEXITY_MATCH" && len(ct.CorrectOptions) != 2 {
			c.fail(where, "COMPLEXITY_MATCH tem par tempo e espaço")
		}
		if ch.TemplateType == "TAG_THE_PATTERN" {
			for _, o := range ct.Options {
				if _, ok := glossary[o]; !ok {
					c.fail(where, "opção %q não está no glossário", o)
				}
			}
		}
	}

	for i, line := range ct.CodeLines {
		if !isASCII(line) {
			c.pending(where, "linha %d fora de ASCII", i+1)
		}
	}
	if words := portugueseIdentifiers(ct.CodeLines); len(words) > 0 {
		c.pending(where, "identificadores em português: %s", strings.Join(words, ", "))
	}
	return &p
}

func (c *checker) checkChallengeText(dir, id, l string, p *payload) {
	name := id + "." + l + ".json"
	where := "trilha/desafios/" + name
	var t challengeText
	err := readJSON(filepath.Join(dir, name), &t)
	if os.IsNotExist(err) {
		c.missingLocale(where, l, "o desafio")
		return
	}
	if err != nil {
		c.fail(where, "%v", err)
		return
	}
	for field, v := range map[string]string{"title": t.Title, "description": t.Description, "explanation": t.Explanation} {
		if strings.TrimSpace(v) == "" {
			c.fail(where, "%s vazio", field)
		}
	}
	if p != nil && len(p.Content.WatchVariables) > 0 && strings.TrimSpace(t.WatchNote) == "" {
		c.fail(where, "watch_note vazio num desafio com watch_variables")
	}
	c.checkText(where, t.Title, t.Description, t.Explanation, t.WatchNote)
}

func (c *checker) checkText(where string, texts ...string) {
	for _, t := range texts {
		if m := markupRe.FindString(t); m != "" {
			c.fail(where, "texto com %q, que o app mostra cru: %.60q", m, t)
		}
	}
}

// Português faltando é erro sempre: é a língua de partida. As outras são pendência.
func (c *checker) missingLocale(where, l, what string) {
	if l == locale.PtBR {
		c.fail(where, "falta %s em português", what)
		return
	}
	c.pending(where, "falta %s em %s", what, l)
}

// portugueseIdentifiers devolve os identificadores do código com palavra da lista,
// ignorando o que está dentro de string e de comentário.
func portugueseIdentifiers(lines []string) []string {
	strOrComment := regexp.MustCompile(`"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'|//.*`)
	found := map[string]bool{}
	for _, line := range lines {
		for _, ident := range identRe.FindAllString(strOrComment.ReplaceAllString(line, ""), -1) {
			for _, w := range splitIdent(ident) {
				if portugueseWords[w] {
					found[ident] = true
				}
			}
		}
	}
	return sortedKeys(found)
}

// splitIdent parte camelCase e snake_case em palavras minúsculas.
func splitIdent(ident string) []string {
	var words []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			words = append(words, strings.ToLower(string(cur)))
			cur = cur[:0]
		}
	}
	for _, r := range ident {
		switch {
		case r == '_':
			flush()
		case unicode.IsUpper(r) && len(cur) > 0 && !unicode.IsUpper(cur[len(cur)-1]):
			flush()
			cur = append(cur, r)
		default:
			cur = append(cur, r)
		}
	}
	flush()
	return words
}

func isASCII(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
