package main

import (
	"strings"
	"testing"
)

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestServerKeysComeFromTheJSONOrLoose(t *testing.T) {
	keys, err := parseServerKeys(envOf(map[string]string{
		"SERVER_KEYS":      `{"jwt_secret":"j","github_client_secret":"g"}`,
		"TRACK_KEY_SECRET": "t",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if keys.JWTSecret != "j" || keys.TrackKeySecret != "t" || keys.GitHubClientSecret != "g" {
		t.Fatalf("chaves erradas: %+v", keys)
	}

	// A mesma chave nas duas fontes com o mesmo valor é o meio da troca, e passa.
	if _, err := parseServerKeys(envOf(map[string]string{
		"SERVER_KEYS": `{"jwt_secret":"j"}`, "JWT_SECRET": "j",
	})); err != nil {
		t.Fatalf("mesmo valor nas duas fontes: %v", err)
	}
}

func TestWeakJWTSecretIsRefusedInProduction(t *testing.T) {
	for name, secret := range map[string]string{
		"vazia":                "",
		"curta":                "curta-demais",
		"de desenvolvimento":   devJWTSecret,
		"marcador do exemplo":  "…",
		"marcador com recheio": "abcdefghijklmnopqrstuvwxyz0123456789…",
	} {
		if checkJWTSecret(secret, true) == nil {
			t.Errorf("%s: chave aceita em produção", name)
		}
	}
	if err := checkJWTSecret("0123456789abcdef0123456789abcdef", true); err != nil {
		t.Fatalf("chave de 32 bytes recusada: %v", err)
	}
	// Fora do Cloud Run vale o padrão de desenvolvimento.
	if err := checkJWTSecret("", false); err != nil {
		t.Fatal(err)
	}
}

func TestServerKeysRefuses(t *testing.T) {
	cases := map[string]map[string]string{
		"valores diferentes":  {"SERVER_KEYS": `{"jwt_secret":"novo"}`, "JWT_SECRET": "velho"},
		"campo desconhecido":  {"SERVER_KEYS": `{"jwt_secert":"j"}`},
		"JSON quebrado":       {"SERVER_KEYS": `{"jwt_secret":`},
		"lixo depois do JSON": {"SERVER_KEYS": `{"jwt_secret":"j"} {"jwt_secret":"k"}`},
		"não é objeto":        {"SERVER_KEYS": `["j"]`},
	}
	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseServerKeys(envOf(env))
			if err == nil {
				t.Fatal("configuração tinha de ser recusada")
			}
			// A mensagem vai para o log: nunca pode trazer o valor de uma chave.
			for _, secret := range []string{"novo", "velho", `"j"`} {
				if strings.Contains(err.Error(), secret) {
					t.Fatalf("erro cita valor de chave: %v", err)
				}
			}
		})
	}
}
