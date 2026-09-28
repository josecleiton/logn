package main

import (
	"encoding/base64"
	"log"
	"os"

	"github.com/josecleiton/logn/backend/internal/storekit"
)

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
