// Package storekittest assina transações e notificações como a App Store assina, com
// uma cadeia gerada na hora. Só para teste: a raiz dela não é a da Apple, e um
// validador de produção a recusa.
package storekittest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"math/big"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	oidLeaf         = asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 6, 11, 1}
	oidIntermediate = asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 6, 2, 1}
)

// Chain é uma raiz, uma intermediária e uma folha, com as extensões da Apple.
type Chain struct {
	Root, Intermediate, Leaf *x509.Certificate
	leafKey                  *ecdsa.PrivateKey
}

// Options mexe na cadeia para os casos negativos.
type Options struct {
	// Nome da raiz. O padrão imita o da Apple, que é o que um atacante faria.
	RootCommonName string
	// Tira a extensão da App Store da folha.
	NoLeafOID bool
	// Tira a extensão WWDR da intermediária.
	NoIntermediateOID bool
}

// NewChain gera uma cadeia nova.
func NewChain(t testing.TB, opts Options) *Chain {
	t.Helper()
	if opts.RootCommonName == "" {
		opts.RootCommonName = "Apple Root CA - G3"
	}
	now := time.Now()

	rootKey := newKey(t)
	rootTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: opts.RootCommonName},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	root := sign(t, rootTmpl, rootTmpl, &rootKey.PublicKey, rootKey)

	interKey := newKey(t)
	interTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: "Apple Worldwide Developer Relations Certification Authority"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	if !opts.NoIntermediateOID {
		interTmpl.ExtraExtensions = []pkix.Extension{{Id: oidIntermediate, Value: []byte{0x05, 0x00}}}
	}
	inter := sign(t, interTmpl, root, &interKey.PublicKey, rootKey)

	leafKey := newKey(t)
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "Prod ECC Mac App Store and iTunes Store Receipt Signing"},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	if !opts.NoLeafOID {
		leafTmpl.ExtraExtensions = []pkix.Extension{{Id: oidLeaf, Value: []byte{0x05, 0x00}}}
	}
	leaf := sign(t, leafTmpl, inter, &leafKey.PublicKey, interKey)

	return &Chain{Root: root, Intermediate: inter, Leaf: leaf, leafKey: leafKey}
}

// Roots é o conjunto que confia só na raiz desta cadeia.
func (c *Chain) Roots() *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AddCert(c.Root)
	return pool
}

// Sign assina `claims` em ES256, com a cadeia no `x5c`.
func (c *Chain) Sign(t testing.TB, claims jwt.MapClaims) string {
	t.Helper()
	return c.SignWith(t, jwt.SigningMethodES256, claims)
}

// SignWith assina com outro método, para provar que só ES256 passa.
func (c *Chain) SignWith(t testing.TB, method jwt.SigningMethod, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(method, claims)
	token.Header["x5c"] = []string{
		base64.StdEncoding.EncodeToString(c.Leaf.Raw),
		base64.StdEncoding.EncodeToString(c.Intermediate.Raw),
		base64.StdEncoding.EncodeToString(c.Root.Raw),
	}
	var key any = c.leafKey
	if method.Alg() == "HS256" {
		key = []byte("not-a-certificate-key")
	}
	s, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("signing: %v", err)
	}
	return s
}

// Transaction é uma compra avulsa válida; o teste sobrescreve o que quiser.
func Transaction(bundleID, productID, originalID, accountToken string) jwt.MapClaims {
	return jwt.MapClaims{
		"originalTransactionId": originalID,
		"transactionId":         originalID,
		"productId":             productID,
		"bundleId":              bundleID,
		"environment":           "Sandbox",
		"type":                  "Non-Consumable",
		"inAppOwnershipType":    "PURCHASED",
		"signedDate":            time.Now().UnixMilli(),
		"appAccountToken":       accountToken,
	}
}

// Notification embrulha uma transação assinada numa notificação V2.
func Notification(bundleID, notificationType, signedTransaction string) jwt.MapClaims {
	return jwt.MapClaims{
		"notificationType": notificationType,
		"notificationUUID": "00000000-0000-4000-8000-000000000001",
		"version":          "2.0",
		"signedDate":       time.Now().UnixMilli(),
		"data": map[string]any{
			"bundleId":              bundleID,
			"environment":           "Sandbox",
			"signedTransactionInfo": signedTransaction,
		},
	}
}

func newKey(t testing.TB) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	return key
}

func sign(t testing.TB, tmpl, parent *x509.Certificate, pub *ecdsa.PublicKey, signer *ecdsa.PrivateKey) *x509.Certificate {
	t.Helper()
	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, pub, signer)
	if err != nil {
		t.Fatalf("certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return cert
}
