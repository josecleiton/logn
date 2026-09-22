package domain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// A trilha empacotada tem de bater com o banco.
//
// O `trail-seed.json` viaja dentro do app para a primeira abertura sem rede, e ele é
// gerado à mão por `just seed-bundle`. Depender de alguém lembrar de rodar isso depois
// de cada migração de conteúdo é o tipo de garantia que falha na quarta vez — e o
// sintoma seria silencioso: um build sai com a trilha de dois nós enquanto o banco tem
// sete, e o app ainda diz "Trilha de tal dia" com a cara de quem está em dia.
//
// Este teste transforma o esquecimento em erro vermelho. Ele não valida conteúdo: só
// pergunta se o que viaja é o que existe.
type trailSeed struct {
	Version     int    `json:"version"`
	GeneratedAt string `json:"generated_at"`
	Nodes       []struct {
		ID string `json:"id"`
	} `json:"nodes"`
	Challenges []struct {
		ID          string          `json:"id"`
		NodeID      string          `json:"node_id"`
		PositionIdx int             `json:"position_idx"`
		Payload     json.RawMessage `json:"payload"`
	} `json:"challenges"`
}

func TestTrailSeedMatchesTheDatabase(t *testing.T) {
	caminho := filepath.Join("..", "..", "..", "ios", "LogNiOS", "LogNiOS", "Resources", "trail-seed.json")

	bytes, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatalf("não achei a trilha empacotada em %s: %v\nrode `just seed-bundle`", caminho, err)
	}

	var semente trailSeed
	if err := json.Unmarshal(bytes, &semente); err != nil {
		t.Fatalf("a trilha empacotada não é JSON válido: %v", err)
	}

	conn := setupTestDB(t)
	defer conn.Close()
	ctx := context.Background()

	// Nós: só os ids, porque é o que decide se a trilha na tela tem a forma certa.
	nosNoBanco := map[string]bool{}
	linhas, err := conn.Query(ctx, "SELECT id FROM skill_nodes")
	if err != nil {
		t.Fatalf("lendo skill_nodes: %v", err)
	}
	for linhas.Next() {
		var id string
		if err := linhas.Scan(&id); err != nil {
			t.Fatalf("lendo id de nó: %v", err)
		}
		nosNoBanco[id] = true
	}
	linhas.Close()

	nosNaSemente := map[string]bool{}
	for _, n := range semente.Nodes {
		nosNaSemente[n.ID] = true
	}
	compararConjuntos(t, "nós", nosNaSemente, nosNoBanco)

	// Desafios: id, nó, posição e uma impressão do payload. Comparar o payload byte a
	// byte daria falso positivo por espaço em branco; o hash do JSON normalizado não.
	type impressao struct {
		nodeID   string
		posicao  int
		conteudo string
	}

	desafiosNoBanco := map[string]impressao{}
	linhas, err = conn.Query(ctx, "SELECT id, node_id, position_idx, payload FROM challenges")
	if err != nil {
		t.Fatalf("lendo challenges: %v", err)
	}
	for linhas.Next() {
		var id, nodeID string
		var pos int
		var payload []byte
		if err := linhas.Scan(&id, &nodeID, &pos, &payload); err != nil {
			t.Fatalf("lendo desafio: %v", err)
		}
		desafiosNoBanco[id] = impressao{nodeID, pos, hashJSON(t, payload)}
	}
	linhas.Close()

	desafiosNaSemente := map[string]impressao{}
	for _, c := range semente.Challenges {
		desafiosNaSemente[c.ID] = impressao{c.NodeID, c.PositionIdx, hashJSON(t, c.Payload)}
	}

	idsSemente := map[string]bool{}
	for id := range desafiosNaSemente {
		idsSemente[id] = true
	}
	idsBanco := map[string]bool{}
	for id := range desafiosNoBanco {
		idsBanco[id] = true
	}
	compararConjuntos(t, "desafios", idsSemente, idsBanco)

	// Mesmos ids: o conteúdo de cada um também tem de bater. Um UPDATE de payload numa
	// migração não muda a lista de ids, e sem esta parte passaria batido.
	for id, noBanco := range desafiosNoBanco {
		naSemente, existe := desafiosNaSemente[id]
		if !existe {
			continue // já reportado acima
		}
		if naSemente != noBanco {
			t.Errorf(
				"%s mudou desde que a trilha foi empacotada (nó/posição/conteúdo)\nrode `just seed-bundle`",
				id,
			)
		}
	}
}

func compararConjuntos(t *testing.T, oQue string, naSemente, noBanco map[string]bool) {
	t.Helper()

	var faltando, sobrando []string
	for id := range noBanco {
		if !naSemente[id] {
			faltando = append(faltando, id)
		}
	}
	for id := range naSemente {
		if !noBanco[id] {
			sobrando = append(sobrando, id)
		}
	}
	sort.Strings(faltando)
	sort.Strings(sobrando)

	if len(faltando) > 0 {
		t.Errorf(
			"%s no banco que não viajam no app: %v\nrode `just seed-bundle`",
			oQue, faltando,
		)
	}
	if len(sobrando) > 0 {
		t.Errorf(
			"%s que viajam no app e não existem mais no banco: %v\nrode `just seed-bundle`",
			oQue, sobrando,
		)
	}
}

// Impressão do JSON com as chaves em ordem, para diferença de formatação não virar
// falso positivo.
func hashJSON(t *testing.T, bruto []byte) string {
	t.Helper()

	var v any
	if err := json.Unmarshal(bruto, &v); err != nil {
		t.Fatalf("payload não é JSON válido: %v", err)
	}
	normalizado, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("renormalizando payload: %v", err)
	}
	soma := sha256.Sum256(normalizado)
	return fmt.Sprintf("%s", hex.EncodeToString(soma[:8]))
}
