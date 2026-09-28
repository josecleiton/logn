// Package storekit confere as transações e notificações assinadas da App Store (JWS).
//
// A confiança sai de uma raiz fixa, a Apple Root CA - G3 embutida no binário. A versão
// anterior comparava o nome da raiz que vinha no próprio token, e qualquer um gera uma
// raiz autoassinada com esse nome: a compra forjada passava, e a notificação forjada
// revogava a compra de outra pessoa.
package storekit

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	_ "embed"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

//go:embed AppleRootCA-G3.pem
var appleRootG3PEM []byte

// Extensões que a Apple põe nos certificados de assinatura da App Store. Uma cadeia
// que chega à raiz certa mas sem elas é de outro serviço da Apple, e não assina compra.
var (
	oidAppStoreLeaf          = asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 6, 11, 1}
	oidAppleWWDRIntermediate = asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 6, 2, 1}
)

// Ambientes da App Store, como vêm no campo `environment`.
const (
	EnvProduction = "Production"
	EnvSandbox    = "Sandbox"
	EnvXcode      = "Xcode"
)

var (
	ErrInvalidSignature = errors.New("storekit: invalid signature")
	ErrWrongBundle      = errors.New("storekit: bundle mismatch")
	ErrWrongEnvironment = errors.New("storekit: environment not accepted")
	ErrRevoked          = errors.New("storekit: transaction revoked")
	ErrNotPurchased     = errors.New("storekit: transaction is not an owned non-consumable purchase")
)

// AppleRoots devolve o conjunto com a Apple Root CA - G3, e só ela.
func AppleRoots() *x509.CertPool {
	pool, err := RootsFromPEM(appleRootG3PEM)
	if err != nil {
		panic("storekit: embedded Apple root is unreadable: " + err.Error())
	}
	return pool
}

// RootsFromPEM monta um conjunto de raízes a partir de PEM, ou de um certificado DER
// solto — o `.cer` que o Xcode exporta da raiz local do StoreKit Testing, que só vale
// em desenvolvimento.
func RootsFromPEM(data []byte) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	if cert, err := x509.ParseCertificate(data); err == nil {
		pool.AddCert(cert)
		return pool, nil
	}
	found := false
	for {
		var block *pem.Block
		block, data = pem.Decode(data)
		if block == nil {
			break
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		pool.AddCert(cert)
		found = true
	}
	if !found {
		return nil, errors.New("no certificate in PEM")
	}
	return pool, nil
}

type Transaction struct {
	OriginalTransactionID string `json:"originalTransactionId"`
	TransactionID         string `json:"transactionId"`
	ProductID             string `json:"productId"`
	BundleID              string `json:"bundleId"`
	Environment           string `json:"environment"`
	Type                  string `json:"type"`
	InAppOwnershipType    string `json:"inAppOwnershipType"`
	SignedDate            int64  `json:"signedDate"`
	RevocationDate        *int64 `json:"revocationDate"`
	RevocationReason      *int   `json:"revocationReason"`
	// O id da conta que comprou, posto pelo app na compra (`appAccountToken`).
	AppAccountToken string `json:"appAccountToken"`
}

type TransactionClaims struct {
	Transaction
	jwt.RegisteredClaims
}

type NotificationData struct {
	AppAppleID            int    `json:"appAppleId"`
	BundleID              string `json:"bundleId"`
	BundleVersion         string `json:"bundleVersion"`
	Environment           string `json:"environment"`
	SignedTransactionInfo string `json:"signedTransactionInfo"`
	Status                int    `json:"status"`
}

type NotificationPayload struct {
	NotificationType string           `json:"notificationType"`
	Subtype          string           `json:"subtype"`
	NotificationUUID string           `json:"notificationUUID"`
	Data             NotificationData `json:"data"`
	Version          string           `json:"version"`
	SignedDate       int64            `json:"signedDate"`
}

type NotificationClaims struct {
	NotificationPayload
	jwt.RegisteredClaims
}

// Config diz em que se confia. Sem raiz, sem bundle ou sem ambiente, o validador não
// nasce: um valor vazio aqui aceitaria tudo ou recusaria tudo em silêncio.
type Config struct {
	BundleID     string
	Environments []string
	Roots        *x509.CertPool
	// Hora da verificação da cadeia. Nulo é a hora de agora.
	Now func() time.Time
}

type Validator struct {
	bundleID     string
	environments map[string]bool
	roots        *x509.CertPool
	now          func() time.Time
}

func NewValidator(cfg Config) (*Validator, error) {
	if cfg.BundleID == "" {
		return nil, errors.New("storekit: bundle id is required")
	}
	if len(cfg.Environments) == 0 {
		return nil, errors.New("storekit: at least one environment is required")
	}
	if cfg.Roots == nil {
		return nil, errors.New("storekit: roots are required")
	}
	envs := make(map[string]bool, len(cfg.Environments))
	for _, e := range cfg.Environments {
		envs[e] = true
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Validator{bundleID: cfg.BundleID, environments: envs, roots: cfg.Roots, now: now}, nil
}

// VerifyTransaction confere assinatura, bundle, ambiente e posse de uma transação.
//
// Uma transação reembolsada assinada antes do reembolso não traz `revocationDate`:
// quem barra o reuso dela é o registro de revogadas no banco, não esta função.
func (v *Validator) VerifyTransaction(jws string) (*Transaction, error) {
	var claims TransactionClaims
	if err := v.parse(jws, &claims); err != nil {
		return nil, err
	}
	tx := claims.Transaction
	if tx.BundleID != v.bundleID {
		return nil, ErrWrongBundle
	}
	if !v.environments[tx.Environment] {
		return nil, ErrWrongEnvironment
	}
	if tx.RevocationDate != nil {
		return nil, ErrRevoked
	}
	// Trilha é compra avulsa, sem Compartilhamento Familiar (spec, seção 2).
	if tx.Type != "Non-Consumable" || tx.InAppOwnershipType != "PURCHASED" {
		return nil, ErrNotPurchased
	}
	if tx.OriginalTransactionID == "" || tx.TransactionID == "" || tx.ProductID == "" {
		return nil, ErrInvalidSignature
	}
	return &tx, nil
}

// VerifyNotification confere uma App Store Server Notification V2.
func (v *Validator) VerifyNotification(signedPayload string) (*NotificationPayload, error) {
	var claims NotificationClaims
	if err := v.parse(signedPayload, &claims); err != nil {
		return nil, err
	}
	n := claims.NotificationPayload
	if n.Data.BundleID != v.bundleID {
		return nil, ErrWrongBundle
	}
	if !v.environments[n.Data.Environment] {
		return nil, ErrWrongEnvironment
	}
	return &n, nil
}

// VerifyNotificationTransaction confere a transação que vem dentro de uma notificação.
// Diferente de VerifyTransaction, aceita a transação revogada: é justamente ela que
// um REFUND traz.
func (v *Validator) VerifyNotificationTransaction(jws string) (*Transaction, error) {
	var claims TransactionClaims
	if err := v.parse(jws, &claims); err != nil {
		return nil, err
	}
	tx := claims.Transaction
	if tx.BundleID != v.bundleID {
		return nil, ErrWrongBundle
	}
	if !v.environments[tx.Environment] {
		return nil, ErrWrongEnvironment
	}
	if tx.OriginalTransactionID == "" {
		return nil, ErrInvalidSignature
	}
	return &tx, nil
}

func (v *Validator) parse(jws string, claims jwt.Claims) error {
	_, err := jwt.ParseWithClaims(jws, claims, v.signingKey, jwt.WithValidMethods([]string{"ES256"}))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSignature, err)
	}
	return nil
}

// signingKey tira a chave da folha, depois de provar que a cadeia do `x5c` chega à raiz
// fixa. A raiz que vem no cabeçalho é ignorada: só conta a do conjunto confiável.
func (v *Validator) signingKey(token *jwt.Token) (any, error) {
	x5c, ok := token.Header["x5c"].([]any)
	if !ok || len(x5c) != 3 {
		return nil, errors.New("x5c must carry leaf, intermediate and root")
	}
	certs := make([]*x509.Certificate, 2)
	for i := range certs {
		s, ok := x5c[i].(string)
		if !ok {
			return nil, fmt.Errorf("x5c[%d] is not a string", i)
		}
		der, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
		if err != nil {
			return nil, fmt.Errorf("x5c[%d]: %w", i, err)
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, fmt.Errorf("x5c[%d]: %w", i, err)
		}
		certs[i] = cert
	}
	leaf, intermediate := certs[0], certs[1]

	intermediates := x509.NewCertPool()
	intermediates.AddCert(intermediate)
	chains, err := leaf.Verify(x509.VerifyOptions{
		Roots:         v.roots,
		Intermediates: intermediates,
		CurrentTime:   v.now(),
		// A folha da App Store não tem uso de servidor TLS, que é o padrão de Verify.
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	})
	if err != nil {
		return nil, fmt.Errorf("certificate chain: %w", err)
	}
	// A cadeia tem de passar pela intermediária do cabeçalho, e só por ela.
	viaIntermediate := false
	for _, chain := range chains {
		if len(chain) == 3 && chain[1].Equal(intermediate) {
			viaIntermediate = true
			break
		}
	}
	if !viaIntermediate {
		return nil, errors.New("certificate chain does not go through the given intermediate")
	}
	if !hasExtension(leaf, oidAppStoreLeaf) {
		return nil, errors.New("leaf is not an App Store signing certificate")
	}
	if !hasExtension(intermediate, oidAppleWWDRIntermediate) {
		return nil, errors.New("intermediate is not an Apple WWDR certificate")
	}

	key, ok := leaf.PublicKey.(*ecdsa.PublicKey)
	if !ok || key.Curve != elliptic.P256() {
		return nil, errors.New("leaf key is not ECDSA P-256")
	}
	return key, nil
}

func hasExtension(cert *x509.Certificate, oid asn1.ObjectIdentifier) bool {
	for _, ext := range cert.Extensions {
		if ext.Id.Equal(oid) {
			return true
		}
	}
	return false
}
