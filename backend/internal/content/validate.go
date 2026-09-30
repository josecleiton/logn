// Package content confere os arquivos das trilhas antes de virarem migração.
//
// A fonte do conteúdo é a pasta trilhas/ do repositório de conteúdo (ver
// exporta_trilha.py e gen_conteudo.py lá): `tracks.json` com as trilhas, e uma pasta
// por trilha, com um arquivo de estrutura por nó e por desafio e um de texto por língua.
// O banco confere a forma do payload com CHECKs, mas só na hora da migração, em
// produção; aqui o erro aparece antes, com o nome do arquivo.
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

	"github.com/josecleiton/logn/backend/internal/googleplay"
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

// track é uma linha de `trilhas/tracks.json`. O slug é o nome da pasta da trilha.
type track struct {
	ID     string `json:"id"`
	Slug   string `json:"slug"`
	Kind   string `json:"kind"`
	Status string `json:"status"`
	Author string `json:"author"`
	// O mesmo id nas duas lojas (ADR 0022).
	StoreProductID *string `json:"store_product_id"`
	// Só vale na criação da trilha: depois, quem abre e fecha a vitrine é o banco
	// (ADR 0014). Ausente é disponível.
	Available *bool  `json:"available"`
	Color     string `json:"color"`
}

type trackText struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

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
	// O texto de cada opção do TRADEOFF_MATCH nesta língua, pelo identificador do
	// payload. Os outros templates não têm: TAG tira o rótulo do glossário.
	OptionLabels map[string]string `json:"option_labels"`
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
	uuidRe  = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	colorRe = regexp.MustCompile(`^#[0-9A-F]{6}$`)
	identRe = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)
	// Texto que o app mostra cru: marcação vira caractere solto na tela. Colchete depois
	// de identificador é índice citado do código (`v[i]`) e passa; solto é marcador de
	// rascunho ou link de markdown.
	markupRe = regexp.MustCompile("[*`]|(?:^|[^A-Za-z0-9_\\])])\\[")
)

var templates = map[string]bool{
	"SPOT_THE_BUG": true, "DRY_RUN": true, "FILL_IN_THE_BLANK": true,
	"COMPLEXITY_MATCH": true, "TAG_THE_PATTERN": true, "TRADEOFF_MATCH": true,
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
	// Ids de nó e de desafio já vistos, com a trilha de cada um. O gerador faz upsert
	// por id: um id repetido entre trilhas sobrescreve o conteúdo da outra no banco.
	nodeOwner      map[string]string
	challengeOwner map[string]string
	// O cartão de origem não é de uma trilha: `challenge_origins` é uma tabela só, e os
	// arquivos dela moram na pasta da gratuita.
	originsDir string
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

// Validate confere as trilhas em root (a pasta com trilhas/ e glossario/). Devolve todos
// os achados; quem chama decide o que é erro pelo campo Pending e pelo modo.
//
// Sem `trilhas/tracks.json`, confere só `trilhas/free`, a gratuita.
func Validate(root string) []Finding {
	trilhas := filepath.Join(root, "trilhas")
	c := &checker{
		nodeOwner:      map[string]string{},
		challengeOwner: map[string]string{},
		originsDir:     filepath.Join(trilhas, "free"),
	}

	slugs := []string{"free"}
	var tracks []track
	err := readJSON(filepath.Join(trilhas, "tracks.json"), &tracks)
	switch {
	case os.IsNotExist(err):
	case err != nil:
		c.fail("trilhas/tracks.json", "%v", err)
		return c.findings
	default:
		slugs = c.checkTracks(trilhas, tracks)
	}

	glossary := map[string]map[string]string{}
	if err := readJSON(filepath.Join(root, "glossario", "tags.json"), &glossary); err != nil && !os.IsNotExist(err) {
		c.fail("glossario/tags.json", "%v", err)
	}
	c.checkGlossary(glossary)

	for _, slug := range slugs {
		dir := filepath.Join(trilhas, slug)
		label := "trilhas/" + slug
		var nodes []node
		if err := readJSON(filepath.Join(dir, "nos.json"), &nodes); err != nil {
			c.fail(label+"/nos.json", "%v", err)
			continue
		}
		nodeIDs := c.checkNodes(label, nodes)
		c.checkNodeTexts(dir, label, nodeIDs)
		c.checkChallenges(dir, label, nodeIDs, glossary)
	}
	return c.findings
}

// checkTracks confere `tracks.json` e o nome de cada trilha nas línguas, e devolve as
// pastas a conferir. Há uma gratuita só, e é ela que o app abre: não sai da vitrine.
func (c *checker) checkTracks(trilhas string, tracks []track) []string {
	var slugs []string
	ids := map[string]bool{}
	seenSlug := map[string]bool{}
	free := 0
	for _, t := range tracks {
		where := "trilhas/tracks.json " + t.Slug
		switch {
		case !slugRe.MatchString(t.Slug):
			c.fail(where, "slug fora de ^[a-z][a-z0-9_]*$")
			continue
		case seenSlug[t.Slug]:
			c.fail(where, "slug repetido")
			continue
		}
		seenSlug[t.Slug] = true
		if !uuidRe.MatchString(t.ID) {
			c.fail(where, "id %q não é uuid", t.ID)
		} else if ids[t.ID] {
			c.fail(where, "id %s repetido", t.ID)
		}
		ids[t.ID] = true
		if t.Status != "active" && t.Status != "discontinued" {
			c.fail(where, "status %q fora de active e discontinued", t.Status)
		}
		if strings.TrimSpace(t.Author) == "" {
			c.fail(where, "sem author")
		}
		if t.Color != "" && !colorRe.MatchString(t.Color) {
			c.fail(where, "color %q fora de ^#[0-9A-F]{6}$", t.Color)
		}
		hasProduct := t.StoreProductID != nil && strings.TrimSpace(*t.StoreProductID) != ""
		switch t.Kind {
		case "free":
			free++
			if hasProduct {
				c.fail(where, "a gratuita não tem store_product_id")
			}
			if t.Available != nil && !*t.Available {
				c.fail(where, "a gratuita não sai da vitrine")
			}
		case "paid":
			if !hasProduct {
				c.fail(where, "trilha paga sem store_product_id")
			} else if !googleplay.ValidProductID(*t.StoreProductID) {
				// O Play aceita só minúscula, dígito, `.` e `_`, começando por letra ou
				// dígito; a App Store aceita isso também.
				c.fail(where, "store_product_id %q não vale no Google Play", *t.StoreProductID)
			}
		default:
			c.fail(where, "kind %q fora de free e paid", t.Kind)
		}
		if info, err := os.Stat(filepath.Join(trilhas, t.Slug)); err != nil || !info.IsDir() {
			c.fail(where, "sem a pasta trilhas/%s", t.Slug)
			continue
		}
		slugs = append(slugs, t.Slug)
	}
	if free != 1 {
		c.fail("trilhas/tracks.json", "precisa de exatamente uma trilha free, tem %d", free)
	}

	for _, l := range locale.Supported {
		name := "tracks." + l + ".json"
		texts := map[string]trackText{}
		err := readJSON(filepath.Join(trilhas, name), &texts)
		if os.IsNotExist(err) {
			c.missingLocale("trilhas/"+name, l, "os nomes das trilhas")
			continue
		}
		if err != nil {
			c.fail("trilhas/"+name, "%v", err)
			continue
		}
		for _, t := range tracks {
			tx, ok := texts[t.ID]
			switch {
			case !ok:
				c.missingLocale("trilhas/"+name+" "+t.Slug, l, "a trilha")
			case strings.TrimSpace(tx.Name) == "":
				c.fail("trilhas/"+name+" "+t.Slug, "nome vazio")
			default:
				c.checkText("trilhas/"+name+" "+t.Slug, tx.Name, tx.Description)
			}
		}
		for id := range texts {
			if !ids[id] {
				c.fail("trilhas/"+name, "texto para a trilha %s, que não existe em tracks.json", id)
			}
		}
	}
	return slugs
}

func (c *checker) checkNodes(label string, nodes []node) map[string]bool {
	ids := map[string]bool{}
	cells := map[[2]int]string{}
	for _, n := range nodes {
		where := label + "/nos.json " + n.ID
		if ids[n.ID] {
			c.fail(where, "id repetido")
		}
		ids[n.ID] = true
		if other, ok := c.nodeOwner[n.ID]; ok && other != label {
			c.fail(where, "id repete o de um nó de %s", other)
		}
		c.nodeOwner[n.ID] = label
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
				c.fail(label+"/nos.json "+n.ID, "pré-requisito %s não existe", p)
			}
		}
	}
	return ids
}

func (c *checker) checkNodeTexts(dir, label string, nodeIDs map[string]bool) {
	for _, l := range locale.Supported {
		name := "nos." + l + ".json"
		texts := map[string]nodeText{}
		err := readJSON(filepath.Join(dir, name), &texts)
		if os.IsNotExist(err) {
			c.missingLocale(label+"/"+name, l, "os nomes dos nós")
			continue
		}
		if err != nil {
			c.fail(label+"/"+name, "%v", err)
			continue
		}
		for id := range nodeIDs {
			t, ok := texts[id]
			switch {
			case !ok:
				c.missingLocale(label+"/"+name+" "+id, l, "o nó")
			case strings.TrimSpace(t.Name) == "":
				c.fail(label+"/"+name+" "+id, "nome vazio")
			default:
				c.checkText(label+"/"+name+" "+id, t.Name, t.Description)
			}
		}
		for id := range texts {
			if !nodeIDs[id] {
				c.fail(label+"/"+name, "texto para o nó %s, que não existe em nos.json", id)
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

func (c *checker) checkChallenges(trilha, label string, nodeIDs map[string]bool, glossary map[string]map[string]string) {
	dir := filepath.Join(trilha, "desafios")
	label += "/desafios/"
	entries, err := os.ReadDir(dir)
	if err != nil {
		c.fail(label, "%v", err)
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
		where := label + e.Name()
		var ch challenge
		if err := readJSON(filepath.Join(dir, e.Name()), &ch); err != nil {
			c.fail(where, "%v", err)
			continue
		}
		if ch.ID != base {
			c.fail(where, "o id dentro (%q) não bate com o nome do arquivo", ch.ID)
		}
		ids[base] = true
		if other, ok := c.challengeOwner[base]; ok {
			c.fail(where, "id repete o de um desafio de %s", other)
		}
		c.challengeOwner[base] = strings.TrimSuffix(label, "/desafios/")
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
			c.checkChallengeText(dir, label, base, l, ch.TemplateType, p)
		}
	}

	for _, t := range texts {
		id, l, _ := strings.Cut(t, ".")
		if !ids[id] {
			c.fail(label+t+".json", "texto de um desafio que não existe")
		}
		if !locale.IsSupported(l) {
			c.fail(label+t+".json", "língua %q não é servida", l)
		}
	}

	c.checkOrigins(label, originsUsed)
}

// checkOrigins confere que toda origem usada por um desafio tem cartão nas três
// línguas. `origens.json` traz o pt-BR e é onde o id nasce; `origens.<locale>.json`
// traz as outras, no mesmo formato de `nos.<locale>.json`. O erro nomeia o desafio, não
// só a origem, porque é o desafio que a pessoa está editando quando lê o achado.
func (c *checker) checkOrigins(label string, originsUsed map[string]string) {
	if len(originsUsed) == 0 {
		return
	}

	cardsByLocale := map[string]map[string]originCard{}

	var base []originEntry
	if err := readJSON(filepath.Join(c.originsDir, "origens.json"), &base); err != nil {
		c.fail("trilhas/free/origens.json", "%v", err)
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
		err := readJSON(filepath.Join(c.originsDir, name), &texts)
		if os.IsNotExist(err) {
			cardsByLocale[l] = map[string]originCard{}
			continue
		}
		if err != nil {
			c.fail("trilhas/free/"+name, "%v", err)
			continue
		}
		cardsByLocale[l] = texts
	}

	for _, challengeID := range sortedKeys(originsUsed) {
		origin := originsUsed[challengeID]
		where := label + challengeID + ".json"
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
	case "TRADEOFF_MATCH":
		// Mesma régua da CHECK da 0053. As opções são identificadores: o texto vai no
		// arquivo de cada língua, em option_labels.
		seen := map[string]bool{}
		for _, o := range ct.Options {
			if !slugRe.MatchString(o) {
				c.fail(where, "opção %q fora de ^[a-z][a-z0-9_]*$: é identificador, o texto vai em option_labels", o)
			}
			if seen[o] {
				c.fail(where, "opção %q repetida", o)
			}
			seen[o] = true
		}
		if len(ct.Options) < 3 {
			c.fail(where, "TRADEOFF_MATCH precisa de ao menos três opções: duas respostas e uma que não é")
		}
		if len(ct.CorrectOptions) != 2 || ct.CorrectOptions[0] == ct.CorrectOptions[1] {
			c.fail(where, "TRADEOFF_MATCH tem par benefício e desvantagem, diferentes")
		}
		for _, o := range ct.CorrectOptions {
			if !seen[o] {
				c.fail(where, "resposta certa %q fora das opções", o)
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

func (c *checker) checkChallengeText(dir, label, id, l, template string, p *payload) {
	name := id + "." + l + ".json"
	where := label + name
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
	c.checkOptionLabels(where, template, p, t.OptionLabels)
}

// checkOptionLabels confere o texto das opções do TRADEOFF_MATCH numa língua: toda opção
// tem texto, nenhum sobra, e dois não se repetem — o Core julga comparando texto, e
// duas opções com o mesmo rótulo seriam a mesma resposta.
func (c *checker) checkOptionLabels(where, template string, p *payload, labels map[string]string) {
	if template != "TRADEOFF_MATCH" {
		if len(labels) > 0 {
			c.fail(where, "option_labels só vale no TRADEOFF_MATCH")
		}
		return
	}
	if p == nil {
		return
	}
	options := map[string]bool{}
	byText := map[string]string{}
	for _, o := range p.Content.Options {
		options[o] = true
		text := strings.TrimSpace(labels[o])
		if text == "" {
			c.fail(where, "falta o texto da opção %q em option_labels", o)
			continue
		}
		if other, ok := byText[strings.ToLower(text)]; ok {
			c.fail(where, "as opções %q e %q têm o mesmo texto", other, o)
		}
		byText[strings.ToLower(text)] = o
		c.checkText(where, text)
	}
	for _, o := range sortedKeys(labels) {
		if !options[o] {
			c.fail(where, "option_labels tem %q, que não é opção do desafio", o)
		}
	}
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
