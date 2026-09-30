package main

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/josecleiton/logn/backend/internal/infrastructure/socialauth"
	"github.com/josecleiton/logn/backend/internal/storekit"
)

// socialVerifiersFromEnv monta um verificador por provedor configurado.
//
// Sem GOOGLE_IOS_CLIENT_ID o login pelo Google fica desligado, e a rota responde
// `provider_disabled`; o servidor sobe do mesmo jeito. O client ID não é segredo, e
// derrubar o deploy por ele tiraria do ar também quem entra por e-mail.
//
// A Apple liga só com as três variáveis da chave do Sign in with Apple (ADR 0017): sem
// a chave não há como revogar o acesso na exclusão da conta, e a Apple exige a
// revogação. Uma parte sem as outras é configuração pela metade, e em produção o
// servidor aborta.
func socialVerifiersFromEnv() (map[string]socialauth.Verifier, map[string]socialauth.Revoker) {
	verifiers := map[string]socialauth.Verifier{}
	revokers := map[string]socialauth.Revoker{}
	if aud := strings.TrimSpace(os.Getenv("GOOGLE_IOS_CLIENT_ID")); aud != "" {
		v, err := socialauth.NewGoogleVerifier(aud)
		if err != nil {
			log.Fatalf("GOOGLE_IOS_CLIENT_ID inválido: %v", err)
		}
		verifiers[socialauth.ProviderGoogle] = v
	} else {
		log.Println("GOOGLE_IOS_CLIENT_ID is not set. Google sign-in is disabled.")
	}

	teamID := strings.TrimSpace(os.Getenv("APPLE_SIGNIN_TEAM_ID"))
	keyID := strings.TrimSpace(os.Getenv("APPLE_SIGNIN_KEY_ID"))
	// No Secret Manager a PEM vem com quebras de verdade; no .env, numa linha só, com
	// `\n` escrito.
	privateKey := strings.ReplaceAll(os.Getenv("APPLE_SIGNIN_PRIVATE_KEY"), `\n`, "\n")
	switch set := btoi(teamID != "") + btoi(keyID != "") + btoi(privateKey != ""); set {
	case 0:
		log.Println("APPLE_SIGNIN_* is not set. Sign in with Apple is disabled.")
	case 3:
		bundleID := strings.TrimSpace(os.Getenv("APPLE_BUNDLE_ID"))
		if bundleID == "" {
			bundleID = "sh.logn.app"
		}
		v, err := socialauth.NewAppleVerifier(bundleID, nil)
		if err != nil {
			log.Fatalf("Sign in with Apple: %v", err)
		}
		r, err := socialauth.NewAppleRevoker(teamID, keyID, bundleID, privateKey, nil)
		if err != nil {
			log.Fatalf("Sign in with Apple: %v", err)
		}
		verifiers[socialauth.ProviderApple] = v
		revokers[socialauth.ProviderApple] = r
	default:
		if os.Getenv("K_SERVICE") != "" {
			log.Fatalf("APPLE_SIGNIN_TEAM_ID, APPLE_SIGNIN_KEY_ID and APPLE_SIGNIN_PRIVATE_KEY must be set together.")
		}
		log.Println("WARNING: APPLE_SIGNIN_* is only partly set. Sign in with Apple is disabled.")
	}
	return verifiers, revokers
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// serverKeys são as chaves que o servidor gera e que ninguém mais lê. No Cloud Run elas
// chegam juntas, num segredo JSON só, em SERVER_KEYS (ADR 0019): um segredo por chave
// passava do free tier do Secret Manager. Fora dele, cada uma pode vir solta, na
// variável de sempre.
type serverKeys struct {
	JWTSecret          string `json:"jwt_secret"`
	TrackKeySecret     string `json:"track_key_secret"`
	GitHubClientSecret string `json:"github_client_secret"`
}

// serverKeysFromEnv junta SERVER_KEYS e as variáveis soltas. A mesma chave nas duas
// fontes com valores diferentes é configuração que ninguém sabe qual vale, e o servidor
// não sobe. Campo desconhecido no JSON também não: é nome escrito errado, e a chave
// que ele devia trazer ficaria vazia sem aviso.
func serverKeysFromEnv() serverKeys {
	keys, err := parseServerKeys(os.Getenv)
	if err != nil {
		log.Fatalf("%v", err)
	}
	return keys
}

// parseServerKeys faz o trabalho de serverKeysFromEnv sobre um `getenv` qualquer. Os
// erros nunca citam valor de chave.
func parseServerKeys(getenv func(string) string) (serverKeys, error) {
	var keys serverKeys
	if raw := getenv("SERVER_KEYS"); raw != "" {
		dec := json.NewDecoder(strings.NewReader(raw))
		dec.DisallowUnknownFields()
		// O erro do decoder pode citar o conteúdo; a mensagem leva só que falhou.
		if err := dec.Decode(&keys); err != nil {
			return serverKeys{}, errors.New("SERVER_KEYS is not a valid JSON object with the known keys")
		}
		if dec.More() {
			return serverKeys{}, errors.New("SERVER_KEYS has trailing content after the JSON object")
		}
	}
	for _, k := range []struct {
		field, env string
		dst        *string
	}{
		{"jwt_secret", "JWT_SECRET", &keys.JWTSecret},
		{"track_key_secret", "TRACK_KEY_SECRET", &keys.TrackKeySecret},
		{"github_client_secret", "GITHUB_CLIENT_SECRET", &keys.GitHubClientSecret},
	} {
		loose := getenv(k.env)
		switch {
		case *k.dst == "":
			*k.dst = loose
		case loose != "" && subtle.ConstantTimeCompare([]byte(*k.dst), []byte(loose)) != 1:
			return serverKeys{}, fmt.Errorf("%s and SERVER_KEYS.%s are both set, with different values", k.env, k.field)
		}
	}
	return keys, nil
}

// devJWTSecret é a chave de sessão de desenvolvimento. Visivelmente falsa, e recusada no
// Cloud Run.
const devJWTSecret = "my-super-secret-logn-key-for-dev"

// minJWTSecretLen é o mínimo aceito em produção. Com o código público, uma chave curta
// ou copiada do exemplo deixa qualquer um forjar sessão e bilhete do GitHub.
const minJWTSecretLen = 32

// jwtSecretFrom devolve a chave da sessão. Em produção sem ela, ou com uma fraca, o
// servidor aborta.
func jwtSecretFrom(keys serverKeys) []byte {
	production := os.Getenv("K_SERVICE") != ""
	if err := checkJWTSecret(keys.JWTSecret, production); err != nil {
		log.Fatalf("%v", err)
	}
	if keys.JWTSecret == "" {
		log.Println("WARNING: JWT_SECRET is not set. Using insecure default for development.")
		return []byte(devJWTSecret)
	}
	return []byte(keys.JWTSecret)
}

// checkJWTSecret recusa, em produção, chave vazia, curta, a de desenvolvimento ou o
// marcador do exemplo da ADR 0019 (`…`).
func checkJWTSecret(secret string, production bool) error {
	if !production {
		return nil
	}
	switch {
	case secret == "":
		return errors.New("JWT_SECRET is not set. Refusing to start in production with an insecure default")
	case len(secret) < minJWTSecretLen, secret == devJWTSecret, strings.Contains(secret, "…"):
		return fmt.Errorf("JWT_SECRET is too weak for production: at least %d random bytes", minJWTSecretLen)
	}
	return nil
}

// githubFromEnv monta a troca e o verificador do GitHub (ADR 0019). Client ID e secret
// vêm juntos: sem nenhum, o GitHub fica desligado; com um só, é deploy pela metade, e
// em produção o servidor aborta.
func githubFromEnv(keys serverKeys, sessionKey []byte) *socialauth.GitHub {
	clientID := strings.TrimSpace(os.Getenv("GITHUB_CLIENT_ID"))
	secret := strings.TrimSpace(keys.GitHubClientSecret)
	switch {
	case clientID == "" && secret == "":
		log.Println("GITHUB_CLIENT_ID is not set. GitHub sign-in is disabled.")
		return nil
	case clientID == "" || secret == "":
		if os.Getenv("K_SERVICE") != "" {
			log.Fatalf("GITHUB_CLIENT_ID and the GitHub client secret must be set together.")
		}
		log.Println("WARNING: GitHub sign-in is only partly configured. It is disabled.")
		return nil
	}
	redirect := strings.TrimSpace(os.Getenv("GITHUB_REDIRECT_URI"))
	if redirect == "" {
		redirect = githubDefaultRedirect
	}
	g, err := socialauth.NewGitHub(clientID, secret, redirect, sessionKey, nil)
	if err != nil {
		log.Fatalf("GitHub sign-in: %v", err)
	}
	return g
}

// githubDefaultRedirect é o retorno que o app intercepta, e que o OAuth App do GitHub
// tem cadastrado como callback. A troca manda o mesmo valor.
const githubDefaultRedirect = "logn://oauth/github"

// devTrackKeySecret é a chave de desenvolvimento de `track_keys`. Visivelmente falsa, e
// o servidor recusa subir com ela no Cloud Run.
var devTrackKeySecret = []byte("logn-dev-track-key-secret-32byte")

// trackKeySecretFrom lê a chave de `track_keys`, 32 bytes em base64. Em produção sem ela
// o servidor aborta, como com JWT_SECRET.
func trackKeySecretFrom(keys serverKeys) []byte {
	raw := keys.TrackKeySecret
	if raw == "" {
		if os.Getenv("K_SERVICE") != "" {
			log.Fatalf("TRACK_KEY_SECRET is not set. Refusing to start in production without it.")
		}
		log.Println("WARNING: TRACK_KEY_SECRET is not set. Using insecure default for development.")
		return devTrackKeySecret
	}
	secret, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(secret) != 32 {
		log.Fatalf("TRACK_KEY_SECRET must be 32 bytes in base64.")
	}
	return secret
}

// storeKitValidatorFromEnv monta o validador das compras.
//
// Produção aceita Production e Sandbox: a App Review compra em Sandbox contra o build
// de produção, e recusar isso é rejeição na revisão. O custo, aceito, é que quem testa
// pelo TestFlight ganha a trilha de verdade.
//
// O ambiente Xcode (StoreKit Testing local) só existe fora do Cloud Run, e só com a raiz
// local exportada do Xcode em APPLE_XCODE_ROOT_CERT. Não há mais atalho que pule a
// verificação da cadeia.
func storeKitValidatorFromEnv() *storekit.Validator {
	production := os.Getenv("K_SERVICE") != ""

	bundleID := os.Getenv("APPLE_BUNDLE_ID")
	if bundleID == "" {
		if production {
			log.Fatalf("APPLE_BUNDLE_ID is not set. Refusing to start in production without it.")
		}
		bundleID = "sh.logn.app"
	}

	cfg := storekit.Config{
		BundleID:     bundleID,
		Environments: []string{storekit.EnvProduction, storekit.EnvSandbox},
		Roots:        storekit.AppleRoots(),
	}
	if path := os.Getenv("APPLE_XCODE_ROOT_CERT"); path != "" {
		if production {
			log.Fatalf("APPLE_XCODE_ROOT_CERT is set in production. The Xcode test root never signs a real purchase.")
		}
		pemBytes, err := os.ReadFile(path)
		if err != nil {
			log.Fatalf("APPLE_XCODE_ROOT_CERT unreadable: %v", err)
		}
		roots, err := storekit.RootsFromPEM(pemBytes)
		if err != nil {
			log.Fatalf("APPLE_XCODE_ROOT_CERT: %v", err)
		}
		cfg.Roots = roots
		cfg.Environments = []string{storekit.EnvXcode}
		log.Println("WARNING: StoreKit validator trusts the local Xcode root. Development only.")
	}

	v, err := storekit.NewValidator(cfg)
	if err != nil {
		log.Fatalf("StoreKit validator: %v", err)
	}
	return v
}
