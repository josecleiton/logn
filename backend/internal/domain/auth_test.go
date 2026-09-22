package domain

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// O `/sync` confiava no `user_id` do corpo do pedido. Agora ele sai daqui, então
// aqui é onde um token forjado tem de morrer.
func TestUserIDFromAccessToken(t *testing.T) {
	JwtSecretKey = []byte("segredo-de-teste")
	const userID = "11111111-2222-3333-4444-555555555555"

	valid, err := GenerateAccessToken(userID)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}
	got, err := UserIDFromAccessToken(valid)
	if err != nil || got != userID {
		t.Fatalf("token válido: got %q err %v, want %q", got, err, userID)
	}

	t.Run("assinado com outro segredo", func(t *testing.T) {
		forged := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id": "00000000-0000-0000-0000-00000000dead",
			"exp":     time.Now().Add(time.Hour).Unix(),
		})
		signed, _ := forged.SignedString([]byte("outro-segredo"))
		if _, err := UserIDFromAccessToken(signed); err == nil {
			t.Fatal("aceitou token assinado com outro segredo")
		}
	})

	t.Run("expirado", func(t *testing.T) {
		expired := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id": userID,
			"exp":     time.Now().Add(-time.Minute).Unix(),
		})
		signed, _ := expired.SignedString(JwtSecretKey)
		if _, err := UserIDFromAccessToken(signed); err == nil {
			t.Fatal("aceitou token expirado")
		}
	})

	t.Run("alg none", func(t *testing.T) {
		none := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
			"user_id": userID,
			"exp":     time.Now().Add(time.Hour).Unix(),
		})
		signed, _ := none.SignedString(jwt.UnsafeAllowNoneSignatureType)
		if _, err := UserIDFromAccessToken(signed); err == nil {
			t.Fatal("aceitou token sem assinatura")
		}
	})

	t.Run("sem user_id", func(t *testing.T) {
		empty := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		signed, _ := empty.SignedString(JwtSecretKey)
		if _, err := UserIDFromAccessToken(signed); err == nil {
			t.Fatal("aceitou token sem dono")
		}
	})

	t.Run("lixo", func(t *testing.T) {
		if _, err := UserIDFromAccessToken("não é um token"); err == nil {
			t.Fatal("aceitou texto qualquer")
		}
	})
}
