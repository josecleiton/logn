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
		"trilhas/free/nos.json": []any{map[string]any{
			"id": nodeID, "row": 0, "col": 0, "required_xp": 0, "prerequisites": []any{}, "topic": "adhoc",
		}},
		"trilhas/free/nos.pt-BR.json": map[string]any{nodeID: map[string]any{"name": "1. Fundamentos", "description": ""}},
		"trilhas/free/nos.en.json":    map[string]any{nodeID: map[string]any{"name": "1. Fundamentos", "description": ""}},
		"trilhas/free/nos.es.json":    map[string]any{nodeID: map[string]any{"name": "1. Fundamentos", "description": ""}},
		"trilhas/free/desafios/ch_1.json": map[string]any{
			"id": "ch_1", "node_id": nodeID, "template_type": "DRY_RUN", "position_idx": 1, "origin": "",
			"payload": map[string]any{
				"content": map[string]any{
					"code_lines":      []any{"int sum = 0;", "for (int i = 0; i < 3; i++) sum += v[i];"},
					"watch_variables": []any{map[string]any{"name": "sum", "value": "—"}},
				},
				"validation": map[string]any{"type": "OUTPUT_MATCH", "expected_string": "6", "trace_cells": 2},
			},
		},
		"trilhas/free/desafios/ch_1.pt-BR.json": text("Soma"),
		"trilhas/free/desafios/ch_1.en.json":    text("Sum"),
		"trilhas/free/desafios/ch_1.es.json":    text("Suma"),
		"trilhas/free/desafios/ch_2.json": map[string]any{
			"id": "ch_2", "node_id": nodeID, "template_type": "TAG_THE_PATTERN", "position_idx": 2, "origin": "",
			"payload": map[string]any{
				"content":    map[string]any{"code_lines": []any{}, "options": []any{"queue", "stack"}, "correct_options": []any{"queue"}},
				"validation": map[string]any{"type": "TAG_MATCH"},
			},
		},
		"trilhas/free/desafios/ch_2.pt-BR.json": text("Fila"),
		"trilhas/free/desafios/ch_2.en.json":    text("Queue"),
		"trilhas/free/desafios/ch_2.es.json":    text("Cola"),
		"trilhas/free/desafios/ch_3.json": map[string]any{
			"id": "ch_3", "node_id": nodeID, "template_type": "TRADEOFF_MATCH", "position_idx": 3, "origin": "",
			"payload": map[string]any{
				"content":    map[string]any{"code_lines": []any{}, "options": []any{"fast", "cheap", "simple"}, "correct_options": []any{"fast", "cheap"}},
				"validation": map[string]any{"type": "TRADEOFF_MATCH"},
			},
		},
		"trilhas/free/desafios/ch_3.pt-BR.json": withLabels(text("Troca"), "Rápido", "Barato", "Simples"),
		"trilhas/free/desafios/ch_3.en.json":    withLabels(text("Trade"), "Fast", "Cheap", "Simple"),
		"trilhas/free/desafios/ch_3.es.json":    withLabels(text("Cambio"), "Rápido", "Barato", "Sencillo"),
		"glossario/tags.json": map[string]any{
			"queue": map[string]any{"pt-BR": "Fila", "en": "Queue", "es": "Cola"},
			"stack": map[string]any{"pt-BR": "Pilha", "en": "Stack", "es": "Pila"},
		},
	}
}

// withLabels põe no texto de um TRADEOFF_MATCH o rótulo de fast, cheap e simple.
func withLabels(text map[string]any, fast, cheap, simple string) map[string]any {
	text["option_labels"] = map[string]any{"fast": fast, "cheap": cheap, "simple": simple}
	return text
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
			dig(f["trilhas/free/desafios/ch_1.json"], "payload", "content")["title"] = "Soma"
		}, "content.title é texto", false},
		{"expected_string com dois tokens", func(f map[string]any) {
			dig(f["trilhas/free/desafios/ch_1.json"], "payload", "validation")["expected_string"] = "6 7"
		}, "mais de um token", false},
		{"variável de watch fora do código", func(f map[string]any) {
			dig(f["trilhas/free/desafios/ch_1.json"], "payload", "content")["watch_variables"] =
				[]any{map[string]any{"name": "total", "value": "0"}}
		}, `"total" não aparece no código`, false},
		{"watch sem nota", func(f map[string]any) {
			delete(f["trilhas/free/desafios/ch_1.en.json"].(map[string]any), "watch_note")
		}, "watch_note vazio", false},
		{"trace_cells fora da faixa", func(f map[string]any) {
			dig(f["trilhas/free/desafios/ch_1.json"], "payload", "validation")["trace_cells"] = 5
		}, "trace_cells entre 1 e 4", false},
		{"markdown no texto", func(f map[string]any) {
			f["trilhas/free/desafios/ch_1.es.json"].(map[string]any)["explanation"] = "Usa `sum`."
		}, "que o app mostra cru", false},
		{"marcador de rascunho", func(f map[string]any) {
			f["trilhas/free/desafios/ch_1.en.json"].(map[string]any)["title"] = "[TO CONFIRM] Sum"
		}, "que o app mostra cru", false},
		{"opção de TAG fora do glossário", func(f map[string]any) {
			dig(f["trilhas/free/desafios/ch_2.json"], "payload", "content")["options"] = []any{"queue", "heap"}
		}, `"heap" não está no glossário`, false},
		{"resposta fora das opções", func(f map[string]any) {
			dig(f["trilhas/free/desafios/ch_2.json"], "payload", "content")["correct_options"] = []any{"bfs"}
		}, "fora das opções", false},
		{"rótulo repetido na mesma língua", func(f map[string]any) {
			dig(f["glossario/tags.json"], "stack")["es"] = "cola"
		}, "repete o de queue", false},
		{"posição repetida no nó", func(f map[string]any) {
			f["trilhas/free/desafios/ch_2.json"].(map[string]any)["position_idx"] = 1
		}, "position_idx 1 repete", false},
		{"id diferente do arquivo", func(f map[string]any) {
			f["trilhas/free/desafios/ch_2.json"].(map[string]any)["id"] = "ch_9"
		}, "não bate com o nome do arquivo", false},
		{"falta português", func(f map[string]any) {
			f["trilhas/free/desafios/ch_1.pt-BR.json"] = nil
		}, "falta o desafio em português", false},
		{"texto órfão", func(f map[string]any) {
			f["trilhas/free/desafios/ch_7.en.json"] = map[string]any{"title": "x", "description": "x", "explanation": "x"}
		}, "texto de um desafio que não existe", false},
		{"campo desconhecido", func(f map[string]any) {
			f["trilhas/free/desafios/ch_2.en.json"].(map[string]any)["tittle"] = "x"
		}, "unknown field", false},
		{"falta espanhol", func(f map[string]any) {
			f["trilhas/free/desafios/ch_2.es.json"] = nil
		}, "falta o desafio em es", true},
		{"rótulo sem inglês", func(f map[string]any) {
			dig(f["glossario/tags.json"], "stack")["en"] = ""
		}, "falta o rótulo em en", true},
		{"identificador em português", func(f map[string]any) {
			dig(f["trilhas/free/desafios/ch_1.json"], "payload", "content")["code_lines"] =
				[]any{"int sum = 0;", "int melhorSoma = sum;"}
		}, "identificadores em português: melhorSoma", true},
		{"código fora de ASCII", func(f map[string]any) {
			dig(f["trilhas/free/desafios/ch_1.json"], "payload", "content")["code_lines"] =
				[]any{"int sum = 0; // soma é zero"}
		}, "linha 1 fora de ASCII", true},
		{"trade-off sem texto de uma opção", func(f map[string]any) {
			delete(dig(f["trilhas/free/desafios/ch_3.en.json"], "option_labels"), "simple")
		}, `falta o texto da opção "simple"`, false},
		{"trade-off com duas opções de mesmo texto", func(f map[string]any) {
			dig(f["trilhas/free/desafios/ch_3.es.json"], "option_labels")["simple"] = "barato"
		}, "têm o mesmo texto", false},
		{"trade-off com rótulo que não é opção", func(f map[string]any) {
			dig(f["trilhas/free/desafios/ch_3.pt-BR.json"], "option_labels")["slow"] = "Lento"
		}, `tem "slow", que não é opção`, false},
		{"trade-off com a mesma resposta duas vezes", func(f map[string]any) {
			dig(f["trilhas/free/desafios/ch_3.json"], "payload", "content")["correct_options"] = []any{"fast", "fast"}
		}, "par benefício e desvantagem, diferentes", false},
		{"trade-off com frase no lugar do identificador", func(f map[string]any) {
			dig(f["trilhas/free/desafios/ch_3.json"], "payload", "content")["options"] = []any{"fast", "cheap", "Mais simples"}
			dig(f["trilhas/free/desafios/ch_3.pt-BR.json"], "option_labels")["Mais simples"] = "Simples"
			delete(dig(f["trilhas/free/desafios/ch_3.pt-BR.json"], "option_labels"), "simple")
			dig(f["trilhas/free/desafios/ch_3.en.json"], "option_labels")["Mais simples"] = "Simple"
			delete(dig(f["trilhas/free/desafios/ch_3.en.json"], "option_labels"), "simple")
			dig(f["trilhas/free/desafios/ch_3.es.json"], "option_labels")["Mais simples"] = "Sencillo"
			delete(dig(f["trilhas/free/desafios/ch_3.es.json"], "option_labels"), "simple")
		}, "é identificador, o texto vai em option_labels", false},
		{"option_labels fora do trade-off", func(f map[string]any) {
			f["trilhas/free/desafios/ch_1.en.json"].(map[string]any)["option_labels"] = map[string]any{"x": "y"}
		}, "option_labels só vale no TRADEOFF_MATCH", false},
		{"nó sem topic", func(f map[string]any) {
			f["trilhas/free/nos.json"].([]any)[0].(map[string]any)["topic"] = ""
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

const (
	freeTrackID = "00000000-0000-0000-0000-000000000000"
	paidTrackID = "00000000-0000-0000-0000-00000000000a"
	paidNodeID  = "a0000000-0000-0000-0000-000000000001"
)

// withPaidTrack junta à trilha mínima o `tracks.json` e uma trilha paga inventada,
// "Trilha T", com um nó e um desafio de ids próprios.
func withPaidTrack() map[string]any {
	files := trail()
	product := "com.example.logn.track.t"
	files["trilhas/tracks.json"] = []any{
		map[string]any{"id": freeTrackID, "slug": "free", "kind": "free", "status": "active", "author": "LogN", "app_store_product_id": nil},
		map[string]any{"id": paidTrackID, "slug": "paga", "kind": "paid", "status": "active", "author": "LogN",
			"app_store_product_id": product, "available": false},
	}
	for _, l := range []string{"pt-BR", "en", "es"} {
		files["trilhas/tracks."+l+".json"] = map[string]any{
			freeTrackID: map[string]any{"name": "Problem Solving"},
			paidTrackID: map[string]any{"name": "Trilha T", "description": "Uma trilha de teste."},
		}
		files["trilhas/paga/nos."+l+".json"] = map[string]any{paidNodeID: map[string]any{"name": "Nó A", "description": ""}}
		files["trilhas/paga/desafios/ch_t01."+l+".json"] = map[string]any{"title": "Soma", "description": "d", "explanation": "e"}
	}
	files["trilhas/paga/nos.json"] = []any{map[string]any{
		"id": paidNodeID, "row": 0, "col": 0, "required_xp": 0, "prerequisites": []any{}, "topic": "adhoc",
	}}
	files["trilhas/paga/desafios/ch_t01.json"] = map[string]any{
		"id": "ch_t01", "node_id": paidNodeID, "template_type": "SPOT_THE_BUG", "position_idx": 1, "origin": "",
		"payload": map[string]any{
			"content":    map[string]any{"code_lines": []any{"int a = 1;", "int b = a + 1;"}},
			"validation": map[string]any{"type": "LINE_MATCH", "correct_line": 2},
		},
	}
	return files
}

func TestTracksArePassedOneByOne(t *testing.T) {
	if f := Validate(write(t, withPaidTrack())); len(f) > 0 {
		t.Fatalf("duas trilhas completas deveriam passar, achou: %v", f)
	}

	cases := []struct {
		name  string
		spoil func(files map[string]any)
		want  string
	}{
		// O gerador faz upsert por id: o repetido sobrescrevia a gratuita no banco.
		{"nó com o id de outra trilha", func(f map[string]any) {
			f["trilhas/paga/nos.json"].([]any)[0].(map[string]any)["id"] = nodeID
			for _, l := range []string{"pt-BR", "en", "es"} {
				f["trilhas/paga/nos."+l+".json"] = map[string]any{nodeID: map[string]any{"name": "Nó A", "description": ""}}
			}
			f["trilhas/paga/desafios/ch_t01.json"].(map[string]any)["node_id"] = nodeID
		}, "repete o de um nó de trilhas/free"},
		{"desafio com o id de outra trilha", func(f map[string]any) {
			for _, suffix := range []string{"", ".pt-BR", ".en", ".es"} {
				f["trilhas/paga/desafios/ch_1"+suffix+".json"] = f["trilhas/paga/desafios/ch_t01"+suffix+".json"]
				f["trilhas/paga/desafios/ch_t01"+suffix+".json"] = nil
			}
			f["trilhas/paga/desafios/ch_1.json"].(map[string]any)["id"] = "ch_1"
		}, "repete o de um desafio de trilhas/free"},
		{"gratuita fora da vitrine", func(f map[string]any) {
			f["trilhas/tracks.json"].([]any)[0].(map[string]any)["available"] = false
		}, "a gratuita não sai da vitrine"},
		{"paga sem produto", func(f map[string]any) {
			f["trilhas/tracks.json"].([]any)[1].(map[string]any)["app_store_product_id"] = nil
		}, "trilha paga sem app_store_product_id"},
		{"trilha sem nome em português", func(f map[string]any) {
			delete(f["trilhas/tracks.pt-BR.json"].(map[string]any), paidTrackID)
		}, "falta a trilha em português"},
		{"trilha sem pasta", func(f map[string]any) {
			f["trilhas/tracks.json"].([]any)[1].(map[string]any)["slug"] = "outra"
		}, "sem a pasta trilhas/outra"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := withPaidTrack()
			tc.spoil(files)
			findings := Validate(write(t, files))
			if len(findings) != 1 {
				t.Fatalf("esperava um achado com %q, veio %d: %v", tc.want, len(findings), findings)
			}
			if !strings.Contains(findings[0].String(), tc.want) {
				t.Errorf("achado %q não fala de %q", findings[0], tc.want)
			}
		})
	}
}

// Índice citado do código não é marcação: as explicações falam de v[i] o tempo todo.
func TestIndexingInTextIsNotMarkup(t *testing.T) {
	files := trail()
	files["trilhas/free/desafios/ch_1.pt-BR.json"].(map[string]any)["explanation"] = "Soma v[i] e depois dist[v[i]]."
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
