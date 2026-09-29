package main

import (
	"encoding/base64"
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

// devTrackKeySecret é a chave de desenvolvimento de `track_keys`. Visivelmente falsa, e
// o servidor recusa subir com ela no Cloud Run.
var devTrackKeySecret = []byte("logn-dev-track-key-secret-32byte")

// trackKeySecretFromEnv lê TRACK_KEY_SECRET, 32 bytes em base64. Em produção sem ela o
// servidor aborta, como com JWT_SECRET.
func trackKeySecretFromEnv() []byte {
	raw := os.Getenv("TRACK_KEY_SECRET")
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
