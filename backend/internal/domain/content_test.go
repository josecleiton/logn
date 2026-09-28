package domain

import (
	"context"
	"encoding/json"
	"testing"
)

func TestAssemblePayloadPutsTheTextBack(t *testing.T) {
	neutral := json.RawMessage(`{
		"content": {"code_lines": ["queue<int> q;"], "options": ["queue", "bfs", "greedy"], "correct_options": ["queue", "bfs"],
		            "watch_variables": [{"name": "q", "value": "1 2"}]},
		"validation": {"type": "TAG_MATCH"}
	}`)
	got, err := AssemblePayload(neutral, ChallengeText{
		Title: "Padrão de Solução", Description: "Marque os dois.", Explanation: "As duas opções descrevem a solução.",
		WatchNote:    "estado inicial",
		OptionLabels: map[string]string{"queue": "Tag A", "bfs": "Tag B"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var p struct {
		Content struct {
			Title          string   `json:"title"`
			Description    string   `json:"description"`
			WatchNote      string   `json:"watch_note"`
			Options        []string `json:"options"`
			CorrectOptions []string `json:"correct_options"`
			CodeLines      []string `json:"code_lines"`
		} `json:"content"`
		Validation struct {
			Type        string `json:"type"`
			Explanation string `json:"explanation"`
		} `json:"validation"`
	}
	if err := json.Unmarshal(got, &p); err != nil {
		t.Fatal(err)
	}
	if p.Content.Title != "Padrão de Solução" || p.Content.Description != "Marque os dois." ||
		p.Content.WatchNote != "estado inicial" || p.Validation.Explanation != "As duas opções descrevem a solução." {
		t.Errorf("texto não voltou: %+v", p)
	}
	// Opção e gabarito ganham o mesmo rótulo; opção sem rótulo fica como está.
	if want := []string{"Tag A", "Tag B", "greedy"}; !equalStrings(p.Content.Options, want) {
		t.Errorf("options = %v, want %v", p.Content.Options, want)
	}
	if want := []string{"Tag A", "Tag B"}; !equalStrings(p.Content.CorrectOptions, want) {
		t.Errorf("correct_options = %v, want %v", p.Content.CorrectOptions, want)
	}
	if p.Content.CodeLines[0] != "queue<int> q;" || p.Validation.Type != "TAG_MATCH" {
		t.Error("a estrutura neutra mudou")
	}
}

func TestAssemblePayloadWithoutWatchNoteOmitsIt(t *testing.T) {
	got, err := AssemblePayload(json.RawMessage(`{"content":{"code_lines":[]},"validation":{"type":"LINE_MATCH"}}`),
		ChallengeText{Title: "t", Description: "d", Explanation: "e"})
	if err != nil {
		t.Fatal(err)
	}
	var p map[string]map[string]json.RawMessage
	json.Unmarshal(got, &p)
	if _, ok := p["content"]["watch_note"]; ok {
		t.Error("watch_note vazio não entra no payload")
	}
}

func TestAssemblePayloadRejectsBrokenPayload(t *testing.T) {
	for _, bad := range []string{`not json`, `{"content":{}}`, `{"content":{"options":"x"},"validation":{}}`} {
		if _, err := AssemblePayload(json.RawMessage(bad), ChallengeText{OptionLabels: map[string]string{"a": "A"}}); err == nil {
			t.Errorf("aceitou %s", bad)
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Um nó só aparece numa língua quando ele e todos os desafios dele estão traduzidos
// nela. O desafio que falta esconde o nó inteiro, e o pré-requisito que aponta para
// nó escondido sai da lista.
func TestContentIsPublishedPerLanguage(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()

	const parent = "90000000-0000-0000-0000-000000000001"
	const child = "90000000-0000-0000-0000-000000000002"
	cleanup := func() {
		conn.Exec(ctx, `DELETE FROM challenges WHERE id LIKE 'test_i18n_%'`)
		conn.Exec(ctx, `DELETE FROM skill_nodes WHERE id IN ($1, $2)`, parent, child)
	}
	cleanup()
	t.Cleanup(cleanup)

	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO skill_nodes (id, track_id, row_idx, col_idx, required_xp, prerequisites, topic) VALUES ($1, '00000000-0000-0000-0000-000000000000', 90, 0, 0, '[]', 'graphs')`, []any{parent}},
		{`INSERT INTO skill_nodes (id, track_id, row_idx, col_idx, required_xp, prerequisites) VALUES ($1, '00000000-0000-0000-0000-000000000000', 91, 0, 0, $2)`, []any{child, `["` + parent + `"]`}},
		{`INSERT INTO skill_node_translations (node_id, locale, name) VALUES ($1, 'pt-BR', 'Pai'), ($1, 'es', 'Padre'), ($2, 'pt-BR', 'Filho'), ($2, 'es', 'Hijo')`, []any{parent, child}},
		{`INSERT INTO challenges (id, node_id, template_type, payload, position_idx) VALUES
		  ('test_i18n_1', $1, 'SPOT_THE_BUG', '{"content":{"code_lines":["int a;"]},"validation":{"type":"LINE_MATCH","correct_line":1}}', 1),
		  ('test_i18n_2', $1, 'SPOT_THE_BUG', '{"content":{"code_lines":["int b;"]},"validation":{"type":"LINE_MATCH","correct_line":1}}', 2),
		  ('test_i18n_3', $2, 'SPOT_THE_BUG', '{"content":{"code_lines":["int c;"]},"validation":{"type":"LINE_MATCH","correct_line":1}}', 1)`, []any{parent, child}},
		// pt-BR completo; es falta o segundo desafio do pai.
		{`INSERT INTO challenge_translations (challenge_id, locale, title, description, explanation) VALUES
		  ('test_i18n_1', 'pt-BR', 'Um', 'd', 'e'), ('test_i18n_2', 'pt-BR', 'Dois', 'd', 'e'), ('test_i18n_3', 'pt-BR', 'Três', 'd', 'e'),
		  ('test_i18n_1', 'es', 'Uno', 'd', 'e'), ('test_i18n_3', 'es', 'Tres', 'd', 'e')`, nil},
	} {
		if _, err := conn.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatalf("semear: %v\n%s", err, q.sql)
		}
	}

	nodesIn := func(locale string) map[string]SkillNode {
		nodes, err := repo.GetSkillNodes(ctx, locale, "")
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]SkillNode{}
		for _, n := range nodes {
			out[n.ID] = n
		}
		return out
	}
	challengesIn := func(locale string) map[string]bool {
		chs, err := repo.GetChallenges(ctx, locale, "")
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]bool{}
		for _, c := range chs {
			out[c.ID] = true
		}
		return out
	}

	pt := nodesIn("pt-BR")
	if pt[parent].Name != "Pai" || pt[parent].Topic != "graphs" || pt[child].Name != "Filho" {
		t.Errorf("pt-BR: %+v", pt)
	}
	if len(pt[child].Prerequisites) != 1 {
		t.Error("em pt-BR o filho segue apontando para o pai")
	}
	if c := challengesIn("pt-BR"); !c["test_i18n_1"] || !c["test_i18n_2"] || !c["test_i18n_3"] {
		t.Errorf("pt-BR devia ter os três desafios: %v", c)
	}

	es := nodesIn("es")
	if _, ok := es[parent]; ok {
		t.Error("em es o pai tem desafio sem tradução e devia sumir")
	}
	if es[child].Name != "Hijo" || len(es[child].Prerequisites) != 0 {
		t.Errorf("em es o filho aparece, sem o pré-requisito escondido: %+v", es[child])
	}
	if c := challengesIn("es"); c["test_i18n_1"] || !c["test_i18n_3"] {
		t.Errorf("em es só o desafio do filho: %v", c)
	}

	if _, ok := nodesIn("en")[child]; ok {
		t.Error("em en não há tradução nenhuma")
	}
}
