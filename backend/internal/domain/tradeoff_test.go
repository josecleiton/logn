package domain

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// TRADEOFF_MATCH no banco (0053): par [benefício, desvantagem] dentro das opções, duas
// respostas diferentes, e ao menos uma opção que não é resposta. O desafio é de manual,
// inventado: "Nó A" da "Trilha T".
func TestTheDatabaseHoldsTheTradeoffShape(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	ctx := context.Background()
	p := seedPaidTrack(t, conn)

	insert := func(id, content string) error {
		payload := `{"content":` + content + `,"validation":{"type":"TRADEOFF_MATCH"}}`
		_, err := conn.Exec(ctx, `INSERT INTO challenges (id, node_id, template_type, payload, position_idx)
			VALUES ($1, $2, 'TRADEOFF_MATCH', $3, 9)`, id, p.sampleNode, payload)
		conn.Exec(ctx, `DELETE FROM challenges WHERE id = $1`, id)
		return err
	}

	for name, content := range map[string]string{
		"só duas opções":      `{"options":["a","b"],"correct_options":["a","b"]}`,
		"uma resposta":        `{"options":["a","b","c"],"correct_options":["a"]}`,
		"resposta fora":       `{"options":["a","b","c"],"correct_options":["a","z"]}`,
		"a mesma resposta 2x": `{"options":["a","b","c"],"correct_options":["a","a"]}`,
		"sem correct_options": `{"options":["a","b","c"]}`,
	} {
		if err := insert("test_tradeoff_bad", content); err == nil || !strings.Contains(err.Error(), "chk_tradeoff_par_beneficio_desvantagem") {
			t.Errorf("%s: o banco aceitou ou recusou por outro motivo: %v", name, err)
		}
	}
	if err := insert("test_tradeoff_ok", `{"options":["a","b","c"],"correct_options":["a","b"]}`); err != nil {
		t.Fatalf("forma certa recusada: %v", err)
	}
}

// O servidor troca identificador por rótulo nas opções e no gabarito, na língua pedida:
// o Core compara texto com texto e não sabe de identificador.
func TestTradeoffOptionsArriveInTheLanguage(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	ctx := context.Background()
	repo := NewRepository(conn)
	p := seedPaidTrack(t, conn)

	if _, err := conn.Exec(ctx, `UPDATE challenges SET template_type = 'TRADEOFF_MATCH', payload = $2 WHERE id = $1`, p.sampleCh,
		`{"content":{"code_lines":[],"options":["fast","cheap","simple"],"correct_options":["fast","cheap"]},"validation":{"type":"TRADEOFF_MATCH"}}`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `UPDATE challenge_translations SET option_labels = $3 WHERE challenge_id = $1 AND locale = $2`, p.sampleCh, "en",
		`{"fast":"Fast","cheap":"Cheap","simple":"Simple"}`); err != nil {
		t.Fatal(err)
	}

	chs, err := repo.GetChallenges(ctx, "en", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range chs {
		if c.ID != p.sampleCh {
			continue
		}
		var payload struct {
			Content struct {
				Options        []string `json:"options"`
				CorrectOptions []string `json:"correct_options"`
			} `json:"content"`
		}
		if err := json.Unmarshal(c.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		if strings.Join(payload.Content.Options, ",") != "Fast,Cheap,Simple" || strings.Join(payload.Content.CorrectOptions, ",") != "Fast,Cheap" {
			t.Fatalf("opções sem rótulo ou fora de ordem: %+v", payload.Content)
		}
		return
	}
	t.Fatal("a amostra não veio")
}
