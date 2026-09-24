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

	"github.com/josecleiton/logn/backend/internal/locale"
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
// pergunta se o que viaja é o que a API serviria, língua por língua.
type trailSeed struct {
	Version     int                      `json:"version"`
	GeneratedAt string                   `json:"generated_at"`
	Locales     map[string]seedTrailJSON `json:"locales"`
}

type seedTrailJSON struct {
	Nodes      []json.RawMessage `json:"nodes"`
	Challenges []json.RawMessage `json:"challenges"`
}

// Tem de acompanhar TRAIL_SEED_VERSION em shared_core/src/domain.rs e VERSION em
// tools/seed_bundle.py.
const trailSeedVersion = 2

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
	if semente.Version != trailSeedVersion {
		t.Fatalf("a trilha empacotada é da versão %d, o app lê a %d\nrode `just seed-bundle`",
			semente.Version, trailSeedVersion)
	}

	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()

	for _, l := range locale.Supported {
		nos, err := repo.GetSkillNodes(ctx, l)
		if err != nil {
			t.Fatalf("lendo nós em %s: %v", l, err)
		}
		desafios, err := repo.GetChallenges(ctx, l)
		if err != nil {
			t.Fatalf("lendo desafios em %s: %v", l, err)
		}

		trilha, veio := semente.Locales[l]
		if !veio {
			// Língua sem nada publicado fica de fora da semente, e o app cai em
			// português. Se o banco já publica algo nela, a semente ficou para trás.
			if len(nos) > 0 {
				t.Errorf("%s já tem %d nó(s) publicados e não viaja no app\nrode `just seed-bundle`", l, len(nos))
			}
			continue
		}

		// A impressão é o JSON inteiro que a API mandaria: um UPDATE que troca só o texto
		// de uma tradução não muda nenhum id, e sem isto passaria batido.
		noBanco := map[string]string{}
		for _, n := range nos {
			noBanco[n.ID] = hashValue(t, n)
		}
		compararImpressoes(t, l+" · nós", impressoes(t, trilha.Nodes), noBanco)

		noBanco = map[string]string{}
		for _, c := range desafios {
			noBanco[c.ID] = hashValue(t, c)
		}
		compararImpressoes(t, l+" · desafios", impressoes(t, trilha.Challenges), noBanco)
	}

	if _, temPortugues := semente.Locales[locale.PtBR]; !temPortugues {
		t.Errorf("a semente não tem português, que é para onde o app cai quando falta a língua dele")
	}
}

// impressoes indexa por id a impressão de cada item da semente.
func impressoes(t *testing.T, itens []json.RawMessage) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, bruto := range itens {
		var comId struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(bruto, &comId); err != nil {
			t.Fatalf("item da semente sem id legível: %v", err)
		}
		out[comId.ID] = hashJSON(t, bruto)
	}
	return out
}

func compararImpressoes(t *testing.T, oQue string, naSemente, noBanco map[string]string) {
	t.Helper()

	var faltando, sobrando, mudaram []string
	for id, h := range noBanco {
		s, existe := naSemente[id]
		switch {
		case !existe:
			faltando = append(faltando, id)
		case s != h:
			mudaram = append(mudaram, id)
		}
	}
	for id := range naSemente {
		if _, existe := noBanco[id]; !existe {
			sobrando = append(sobrando, id)
		}
	}
	sort.Strings(faltando)
	sort.Strings(sobrando)
	sort.Strings(mudaram)

	if len(faltando) > 0 {
		t.Errorf("%s no banco que não viajam no app: %v\nrode `just seed-bundle`", oQue, faltando)
	}
	if len(sobrando) > 0 {
		t.Errorf("%s que viajam no app e não existem mais no banco: %v\nrode `just seed-bundle`", oQue, sobrando)
	}
	if len(mudaram) > 0 {
		t.Errorf("%s que mudaram desde que a trilha foi empacotada: %v\nrode `just seed-bundle`", oQue, mudaram)
	}
}

// hashValue tira a impressão de um valor como a API o serializaria.
func hashValue(t *testing.T, v any) string {
	t.Helper()
	bruto, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("serializando: %v", err)
	}
	return hashJSON(t, bruto)
}

// Impressão do JSON com as chaves em ordem, para diferença de formatação não virar
// falso positivo.
func hashJSON(t *testing.T, bruto []byte) string {
	t.Helper()

	var v any
	if err := json.Unmarshal(bruto, &v); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	normalizado, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("renormalizando: %v", err)
	}
	soma := sha256.Sum256(normalizado)
	return fmt.Sprintf("%s", hex.EncodeToString(soma[:8]))
}
