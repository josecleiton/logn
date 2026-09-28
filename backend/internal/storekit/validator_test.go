package storekit_test

import (
	"errors"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/josecleiton/logn/backend/internal/storekit"
	"github.com/josecleiton/logn/backend/internal/storekit/storekittest"
)

const bundle = "com.example.logn"

func validator(t *testing.T, chain *storekittest.Chain) *storekit.Validator {
	t.Helper()
	v, err := storekit.NewValidator(storekit.Config{
		BundleID:     bundle,
		Environments: []string{storekit.EnvProduction, storekit.EnvSandbox},
		Roots:        chain.Roots(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestAValidPurchasePasses(t *testing.T) {
	chain := storekittest.NewChain(t, storekittest.Options{})
	jws := chain.Sign(t, storekittest.Transaction(bundle, "track.a", "1000", "acc"))

	tx, err := validator(t, chain).VerifyTransaction(jws)
	if err != nil {
		t.Fatalf("valid purchase refused: %v", err)
	}
	if tx.OriginalTransactionID != "1000" || tx.ProductID != "track.a" || tx.AppAccountToken != "acc" {
		t.Fatalf("unexpected transaction: %+v", tx)
	}
}

// O ataque da versão anterior: uma raiz própria com o nome da raiz da Apple.
func TestARootNamedLikeApplesIsNotApples(t *testing.T) {
	forged := storekittest.NewChain(t, storekittest.Options{RootCommonName: "Apple Root CA - G3"})
	jws := forged.Sign(t, storekittest.Transaction(bundle, "track.a", "1000", "acc"))

	v, err := storekit.NewValidator(storekit.Config{
		BundleID:     bundle,
		Environments: []string{storekit.EnvProduction, storekit.EnvSandbox},
		Roots:        storekit.AppleRoots(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.VerifyTransaction(jws); !errors.Is(err, storekit.ErrInvalidSignature) {
		t.Fatalf("forged chain accepted, err=%v", err)
	}
	if _, err := v.VerifyNotification(forged.Sign(t, storekittest.Notification(bundle, "REFUND", jws))); !errors.Is(err, storekit.ErrInvalidSignature) {
		t.Fatalf("forged notification accepted, err=%v", err)
	}
}

func TestAChainFromAnotherTrustedRootIsRefused(t *testing.T) {
	trusted := storekittest.NewChain(t, storekittest.Options{})
	other := storekittest.NewChain(t, storekittest.Options{})
	jws := other.Sign(t, storekittest.Transaction(bundle, "track.a", "1000", "acc"))

	if _, err := validator(t, trusted).VerifyTransaction(jws); !errors.Is(err, storekit.ErrInvalidSignature) {
		t.Fatalf("chain from another root accepted, err=%v", err)
	}
}

func TestCertificatesWithoutAppleExtensionsAreRefused(t *testing.T) {
	for name, opts := range map[string]storekittest.Options{
		"leaf":         {NoLeafOID: true},
		"intermediate": {NoIntermediateOID: true},
	} {
		t.Run(name, func(t *testing.T) {
			chain := storekittest.NewChain(t, opts)
			jws := chain.Sign(t, storekittest.Transaction(bundle, "track.a", "1000", "acc"))
			if _, err := validator(t, chain).VerifyTransaction(jws); !errors.Is(err, storekit.ErrInvalidSignature) {
				t.Fatalf("missing %s extension accepted, err=%v", name, err)
			}
		})
	}
}

func TestOnlyES256IsAccepted(t *testing.T) {
	chain := storekittest.NewChain(t, storekittest.Options{})
	jws := chain.SignWith(t, jwt.SigningMethodHS256, storekittest.Transaction(bundle, "track.a", "1000", "acc"))
	if _, err := validator(t, chain).VerifyTransaction(jws); !errors.Is(err, storekit.ErrInvalidSignature) {
		t.Fatalf("HS256 accepted, err=%v", err)
	}
}

func TestTransactionFieldsAreChecked(t *testing.T) {
	chain := storekittest.NewChain(t, storekittest.Options{})
	v := validator(t, chain)

	cases := map[string]struct {
		mutate func(jwt.MapClaims)
		want   error
	}{
		"other bundle":        {func(c jwt.MapClaims) { c["bundleId"] = "com.example.other" }, storekit.ErrWrongBundle},
		"xcode environment":   {func(c jwt.MapClaims) { c["environment"] = storekit.EnvXcode }, storekit.ErrWrongEnvironment},
		"revoked":             {func(c jwt.MapClaims) { c["revocationDate"] = 1 }, storekit.ErrRevoked},
		"family shared":       {func(c jwt.MapClaims) { c["inAppOwnershipType"] = "FAMILY_SHARED" }, storekit.ErrNotPurchased},
		"consumable":          {func(c jwt.MapClaims) { c["type"] = "Consumable" }, storekit.ErrNotPurchased},
		"without transaction": {func(c jwt.MapClaims) { c["originalTransactionId"] = "" }, storekit.ErrInvalidSignature},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			claims := storekittest.Transaction(bundle, "track.a", "1000", "acc")
			tc.mutate(claims)
			if _, err := v.VerifyTransaction(chain.Sign(t, claims)); !errors.Is(err, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, err)
			}
		})
	}
}

// O REFUND traz a transação já revogada; é a notificação que tem de aceitá-la.
func TestANotificationCarriesTheRevokedTransaction(t *testing.T) {
	chain := storekittest.NewChain(t, storekittest.Options{})
	v := validator(t, chain)
	claims := storekittest.Transaction(bundle, "track.a", "1000", "acc")
	claims["revocationDate"] = 1
	inner := chain.Sign(t, claims)

	n, err := v.VerifyNotification(chain.Sign(t, storekittest.Notification(bundle, "REFUND", inner)))
	if err != nil {
		t.Fatalf("notification refused: %v", err)
	}
	tx, err := v.VerifyNotificationTransaction(n.Data.SignedTransactionInfo)
	if err != nil || tx.OriginalTransactionID != "1000" {
		t.Fatalf("inner transaction: tx=%+v err=%v", tx, err)
	}
}

func TestTheValidatorNeedsItsConfiguration(t *testing.T) {
	roots := storekit.AppleRoots()
	for name, cfg := range map[string]storekit.Config{
		"bundle":      {Environments: []string{storekit.EnvProduction}, Roots: roots},
		"environment": {BundleID: bundle, Roots: roots},
		"roots":       {BundleID: bundle, Environments: []string{storekit.EnvProduction}},
	} {
		if _, err := storekit.NewValidator(cfg); err == nil {
			t.Errorf("validator built without %s", name)
		}
	}
}
