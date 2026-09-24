package content

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const nodeID = "10000000-0000-0000-0000-000000000001"

// trail é uma trilha mínima e completa nas três línguas: um nó, um DRY_RUN e um TAG.
// Cada teste estraga uma coisa e confere que a conferência acha exatamente ela.
func trail() map[string]any {
	text := func(title string) map[string]any {
		return map[string]any{"title": title, "description": "Enunciado.", "explanation": "Porque sim.", "watch_note": "Nota."}
	}
	return map[string]any{
		"trilha/nos.json": []any{map[string]any{
			"id": nodeID, "row": 0, "col": 0, "required_xp": 0, "prerequisites": []any{}, "topic": "adhoc",
		}},
		"trilha/nos.pt-BR.json": map[string]any{nodeID: map[string]any{"name": "Nó A", "description": ""}},
		"trilha/nos.en.json":    map[string]any{nodeID: map[string]any{"name": "Nó A", "description": ""}},
		"trilha/nos.es.json":    map[string]any{nodeID: map[string]any{"name": "Nó A", "description": ""}},
		"trilha/desafios/ch_1.json": map[string]any{
			"id": "ch_1", "node_id": nodeID, "template_type": "DRY_RUN", "position_idx": 1, "origin": "",
			"payload": map[string]any{
				"content": map[string]any{
					"code_lines":      []any{"int sum = 0;", "for (int i = 0; i < 3; i++) sum += v[i];"},
					"watch_variables": []any{map[string]any{"name": "sum", "value": "—"}},
				},
				"validation": map[string]any{"type": "OUTPUT_MATCH", "expected_string": "6", "trace_cells": 2},
			},
		},
		"trilha/desafios/ch_1.pt-BR.json": text("Soma"),
		"trilha/desafios/ch_1.en.json":    text("Sum"),
		"trilha/desafios/ch_1.es.json":    text("Suma"),
		"trilha/desafios/ch_2.json": map[string]any{
			"id": "ch_2", "node_id": nodeID, "template_type": "TAG_THE_PATTERN", "position_idx": 2, "origin": "",
			"payload": map[string]any{
				"content":    map[string]any{"code_lines": []any{}, "options": []any{"queue", "stack"}, "correct_options": []any{"queue"}},
				"validation": map[string]any{"type": "TAG_MATCH"},
			},
		},
		"trilha/desafios/ch_2.pt-BR.json": text("Fila"),
		"trilha/desafios/ch_2.en.json":    text("Queue"),
		"trilha/desafios/ch_2.es.json":    text("Cola"),
		"glossario/tags.json": map[string]any{
			"queue": map[string]any{"pt-BR": "Fila", "en": "Queue", "es": "Cola"},
			"stack": map[string]any{"pt-BR": "Pilha", "en": "Stack", "es": "Pila"},
		},
	}
}

func write(t *testing.T, files map[string]any) string {
	t.Helper()
	root := t.TempDir()
	for path, v := range files {
		if v == nil {
			continue
		}
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(v)
		if err := os.WriteFile(full, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func dig(v any, keys ...string) map[string]any {
	m := v.(map[string]any)
	for _, k := range keys {
		m = m[k].(map[string]any)
	}
	return m
}

func TestCompleteTrailPasses(t *testing.T) {
	if f := Validate(write(t, trail())); len(f) > 0 {
		t.Fatalf("trilha completa deveria passar, achou: %v", f)
	}
}

func TestEachRuleIsCaught(t *testing.T) {
	cases := []struct {
		name    string
		spoil   func(files map[string]any)
		want    string
		pending bool
	}{
		{"texto de payload no arquivo neutro", func(f map[string]any) {
			dig(f["trilha/desafios/ch_1.json"], "payload", "content")["title"] = "Soma"
		}, "content.title é texto", false},
		{"expected_string com dois tokens", func(f map[string]any) {
			dig(f["trilha/desafios/ch_1.json"], "payload", "validation")["expected_string"] = "6 7"
		}, "mais de um token", false},
		{"variável de watch fora do código", func(f map[string]any) {
			dig(f["trilha/desafios/ch_1.json"], "payload", "content")["watch_variables"] =
				[]any{map[string]any{"name": "total", "value": "0"}}
		}, `"total" não aparece no código`, false},
		{"watch sem nota", func(f map[string]any) {
			delete(f["trilha/desafios/ch_1.en.json"].(map[string]any), "watch_note")
		}, "watch_note vazio", false},
		{"trace_cells fora da faixa", func(f map[string]any) {
			dig(f["trilha/desafios/ch_1.json"], "payload", "validation")["trace_cells"] = 5
		}, "trace_cells entre 1 e 4", false},
		{"markdown no texto", func(f map[string]any) {
			f["trilha/desafios/ch_1.es.json"].(map[string]any)["explanation"] = "Usa `sum`."
		}, "que o app mostra cru", false},
		{"marcador de rascunho", func(f map[string]any) {
			f["trilha/desafios/ch_1.en.json"].(map[string]any)["title"] = "[TO CONFIRM] Sum"
		}, "que o app mostra cru", false},
		{"opção de TAG fora do glossário", func(f map[string]any) {
			dig(f["trilha/desafios/ch_2.json"], "payload", "content")["options"] = []any{"queue", "heap"}
		}, `"heap" não está no glossário`, false},
		{"resposta fora das opções", func(f map[string]any) {
			dig(f["trilha/desafios/ch_2.json"], "payload", "content")["correct_options"] = []any{"bfs"}
		}, "fora das opções", false},
		{"rótulo repetido na mesma língua", func(f map[string]any) {
			dig(f["glossario/tags.json"], "stack")["es"] = "cola"
		}, "repete o de queue", false},
		{"posição repetida no nó", func(f map[string]any) {
			f["trilha/desafios/ch_2.json"].(map[string]any)["position_idx"] = 1
		}, "position_idx 1 repete", false},
		{"id diferente do arquivo", func(f map[string]any) {
			f["trilha/desafios/ch_2.json"].(map[string]any)["id"] = "ch_9"
		}, "não bate com o nome do arquivo", false},
		{"falta português", func(f map[string]any) {
			f["trilha/desafios/ch_1.pt-BR.json"] = nil
		}, "falta o desafio em português", false},
		{"texto órfão", func(f map[string]any) {
			f["trilha/desafios/ch_7.en.json"] = map[string]any{"title": "x", "description": "x", "explanation": "x"}
		}, "texto de um desafio que não existe", false},
		{"campo desconhecido", func(f map[string]any) {
			f["trilha/desafios/ch_2.en.json"].(map[string]any)["tittle"] = "x"
		}, "unknown field", false},
		{"falta espanhol", func(f map[string]any) {
			f["trilha/desafios/ch_2.es.json"] = nil
		}, "falta o desafio em es", true},
		{"rótulo sem inglês", func(f map[string]any) {
			dig(f["glossario/tags.json"], "stack")["en"] = ""
		}, "falta o rótulo em en", true},
		{"identificador em português", func(f map[string]any) {
			dig(f["trilha/desafios/ch_1.json"], "payload", "content")["code_lines"] =
				[]any{"int sum = 0;", "int melhorSoma = sum;"}
		}, "identificadores em português: melhorSoma", true},
		{"código fora de ASCII", func(f map[string]any) {
			dig(f["trilha/desafios/ch_1.json"], "payload", "content")["code_lines"] =
				[]any{"int sum = 0; // soma é zero"}
		}, "linha 1 fora de ASCII", true},
		{"nó sem topic", func(f map[string]any) {
			f["trilha/nos.json"].([]any)[0].(map[string]any)["topic"] = ""
		}, "sem topic", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := trail()
			tc.spoil(files)
			findings := Validate(write(t, files))
			if len(findings) != 1 {
				t.Fatalf("esperava um achado com %q, veio %d: %v", tc.want, len(findings), findings)
			}
			if !strings.Contains(findings[0].String(), tc.want) {
				t.Errorf("achado %q não fala de %q", findings[0], tc.want)
			}
			if findings[0].Pending != tc.pending {
				t.Errorf("pendente = %v, esperava %v", findings[0].Pending, tc.pending)
			}
		})
	}
}

// Índice citado do código não é marcação: as explicações falam de v[i] o tempo todo.
func TestIndexingInTextIsNotMarkup(t *testing.T) {
	files := trail()
	files["trilha/desafios/ch_1.pt-BR.json"].(map[string]any)["explanation"] = "Soma v[i] e depois dist[v[i]]."
	if f := Validate(write(t, files)); len(f) > 0 {
		t.Fatalf("índice no texto não deveria acusar: %v", f)
	}
}

func TestSplitIdent(t *testing.T) {
	for in, want := range map[string]string{
		"maiorSubida": "maior subida", "is_same_set": "is same set", "MAXN": "maxn", "UnionFind": "union find",
	} {
		if got := strings.Join(splitIdent(in), " "); got != want {
			t.Errorf("splitIdent(%q) = %q, esperava %q", in, got, want)
		}
	}
}
