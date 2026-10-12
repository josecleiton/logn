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
	Origins    []json.RawMessage `json:"origins"`
}

// Tem de acompanhar TRAIL_SEED_VERSION em shared_core/src/domain.rs e VERSION em
// tools/seed_bundle.py. Subiu de 2 para 3 quando os cartões de origem passaram a
// viajar dentro da semente.
const trailSeedVersion = 3

func TestTrailSeedMatchesTheDatabase(t *testing.T) {
	path := filepath.Join("..", "..", "..", "ios", "LogNiOS", "LogNiOS", "Resources", "trail-seed.json")

	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("não achei a trilha empacotada em %s: %v\nrode `just seed-bundle`", path, err)
	}

	var seed trailSeed
	if err := json.Unmarshal(bytes, &seed); err != nil {
		t.Fatalf("a trilha empacotada não é JSON válido: %v", err)
	}
	if seed.Version != trailSeedVersion {
		t.Fatalf("a trilha empacotada é da versão %d, o app lê a %d\nrode `just seed-bundle`",
			seed.Version, trailSeedVersion)
	}

	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()

	for _, l := range locale.Supported {
		nodes, err := repo.GetSkillNodes(ctx, l, "")
		if err != nil {
			t.Fatalf("lendo nós em %s: %v", l, err)
		}
		challenges, err := repo.GetChallenges(ctx, l, "")
		if err != nil {
			t.Fatalf("lendo desafios em %s: %v", l, err)
		}
		origins, err := repo.GetOriginCards(ctx, l)
		if err != nil {
			t.Fatalf("lendo origens em %s: %v", l, err)
		}

		trail, found := seed.Locales[l]
		if !found {
			// Língua sem nada publicado fica de fora da semente, e o app cai em
			// português. Se o banco já publica algo nela, a semente ficou para trás.
			if len(nodes) > 0 {
				t.Errorf("%s já tem %d nó(s) publicados e não viaja no app\nrode `just seed-bundle`", l, len(nodes))
			}
			continue
		}

		// A impressão é o JSON inteiro que a API mandaria: um UPDATE que troca só o texto
		// de uma tradução não muda nenhum id, e sem isto passaria batido.
		inDB := map[string]string{}
		for _, n := range nodes {
			inDB[n.ID] = hashValue(t, n)
		}
		compareFingerprints(t, l+" · nós", fingerprints(t, trail.Nodes), inDB)

		inDB = map[string]string{}
		for _, c := range challenges {
			inDB[c.ID] = hashValue(t, c)
		}
		compareFingerprints(t, l+" · desafios", fingerprints(t, trail.Challenges), inDB)

		inDB = map[string]string{}
		for _, o := range origins {
			inDB[o.ID] = hashValue(t, o)
		}
		compareFingerprints(t, l+" · origens", fingerprints(t, trail.Origins), inDB)
	}

	if _, hasPortuguese := seed.Locales[locale.PtBR]; !hasPortuguese {
		t.Errorf("a semente não tem português, que é para onde o app cai quando falta a língua dele")
	}
}

// fingerprints indexa por id a impressão de cada item da semente.
func fingerprints(t *testing.T, items []json.RawMessage) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, raw := range items {
		var withID struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &withID); err != nil {
			t.Fatalf("item da semente sem id legível: %v", err)
		}
		out[withID.ID] = hashJSON(t, raw)
	}
	return out
}

func compareFingerprints(t *testing.T, label string, inSeed, inDB map[string]string) {
	t.Helper()

	var missing, extra, changed []string
	for id, h := range inDB {
		s, ok := inSeed[id]
		switch {
		case !ok:
			missing = append(missing, id)
		case s != h:
			changed = append(changed, id)
		}
	}
	for id := range inSeed {
		if _, ok := inDB[id]; !ok {
			extra = append(extra, id)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	sort.Strings(changed)

	if len(missing) > 0 {
		t.Errorf("%s no banco que não viajam no app: %v\nrode `just seed-bundle`", label, missing)
	}
	if len(extra) > 0 {
		t.Errorf("%s que viajam no app e não existem mais no banco: %v\nrode `just seed-bundle`", label, extra)
	}
	if len(changed) > 0 {
		t.Errorf("%s que mudaram desde que a trilha foi empacotada: %v\nrode `just seed-bundle`", label, changed)
	}
}

// hashValue tira a impressão de um valor como a API o serializaria.
func hashValue(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("serializando: %v", err)
	}
	return hashJSON(t, raw)
}

// Impressão do JSON com as chaves em ordem, para diferença de formatação não virar
// falso positivo.
func hashJSON(t *testing.T, raw []byte) string {
	t.Helper()

	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	normalized, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("renormalizando: %v", err)
	}
	sum := sha256.Sum256(normalized)
	return fmt.Sprintf("%s", hex.EncodeToString(sum[:8]))
}
