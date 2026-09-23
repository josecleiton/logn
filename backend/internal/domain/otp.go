package domain

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"
)

// Propósitos aceitos para um OTP. Era texto livre: qualquer string de até 50
// caracteres virava linha em `otps`.
const (
	OTPPurposeVerifyEmail   = "verify_email"
	OTPPurposeResetPassword = "reset_password"
)

func ValidOTPPurpose(purpose string) bool {
	return purpose == OTPPurposeVerifyEmail || purpose == OTPPurposeResetPassword
}

// OTPMaxAttempts é quantos códigos errados um OTP aguenta antes de morrer.
//
// Sem limite, seis dígitos se varriam inteiros pelo `verify-otp` dentro dos quinze
// minutos de validade. Com cinco tentativas por código e um código por OTPResendCooldown,
// o atacante precisa de meses por conta.
const OTPMaxAttempts = 5

// OTPResendCooldown é o intervalo mínimo entre dois envios para o mesmo e-mail e
// propósito. Segura o disparo de e-mail para um endereço só e o atacante que pede
// código novo a cada cinco erros.
const OTPResendCooldown = 60 * time.Second

// ErrOTPCooldown sinaliza pedido de código antes do intervalo mínimo.
var ErrOTPCooldown = errors.New("otp requested too soon")

func GenerateOTP() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	return fmt.Sprintf("%06d", n.Int64())
}

// otpHash é o que vai para o banco no lugar do código.
//
// HMAC, e não SHA-256 puro: com um milhão de códigos possíveis, um hash sem chave se
// inverte na hora. A chave é a mesma do JWT porque quem tem uma já tem a outra; e-mail
// e propósito entram para que o mesmo código em duas linhas não dê o mesmo hash.
func otpHash(email, purpose, code string) string {
	mac := hmac.New(sha256.New, JwtSecretKey)
	mac.Write([]byte("otp\x00" + email + "\x00" + purpose + "\x00" + code))
	return hex.EncodeToString(mac.Sum(nil))
}

// SaveOTP grava um código novo para o e-mail e zera as tentativas.
//
// Devolve ErrOTPCooldown se o anterior saiu há menos de OTPResendCooldown. Sem esse
// intervalo, o endpoint mandava e-mail para qualquer endereço sem parar, e cada pedido
// ainda invalidava o código que o dono de verdade tinha acabado de receber.
func (r *Repository) SaveOTP(ctx context.Context, email, code, purpose string, duration time.Duration) error {
	expiresAt := time.Now().Add(duration)
	query := `
		INSERT INTO otps (email, purpose, code_hash, expires_at, attempts, sent_at)
		VALUES ($1, $2, $3, $4, 0, CURRENT_TIMESTAMP)
		ON CONFLICT (email, purpose)
		DO UPDATE SET code_hash = EXCLUDED.code_hash, expires_at = EXCLUDED.expires_at,
		              attempts = 0, sent_at = CURRENT_TIMESTAMP
		WHERE otps.sent_at <= CURRENT_TIMESTAMP - make_interval(secs => $5)
	`
	tag, err := r.db.Exec(ctx, query,
		email, purpose, otpHash(email, purpose, code), expiresAt, OTPResendCooldown.Seconds())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrOTPCooldown
	}
	return nil
}

// CheckOTP diz se o código confere, sem gastá-lo, e conta a tentativa se não conferir.
//
// Um único UPDATE faz as duas coisas: com o check separado do incremento, cem pedidos
// em paralelo liam o mesmo `attempts` e passavam todos pelo limite.
func (r *Repository) CheckOTP(ctx context.Context, email, code, purpose string) (bool, error) {
	query := `
		UPDATE otps
		SET attempts = attempts + CASE WHEN code_hash = $3 THEN 0 ELSE 1 END
		WHERE email = $1 AND purpose = $2
		  AND expires_at > CURRENT_TIMESTAMP
		  AND attempts < $4
		RETURNING code_hash = $3
	`
	var match bool
	err := r.db.QueryRow(ctx, query, email, purpose, otpHash(email, purpose, code), OTPMaxAttempts).Scan(&match)
	if err != nil {
		return false, err // pgx.ErrNoRows: sem código, expirado ou esgotado
	}
	return match, nil
}

// ConsumeOTP confere o código e o apaga, numa operação que só um pedido ganha.
//
// Antes era check e depois delete: duas requisições com o mesmo código passavam as
// duas no intervalo entre um e outro. Agora quem apaga a linha é quem ganhou.
func (r *Repository) ConsumeOTP(ctx context.Context, email, code, purpose string) (bool, error) {
	valid, err := r.CheckOTP(ctx, email, code, purpose)
	if err != nil || !valid {
		return false, err
	}

	tag, err := r.db.Exec(ctx, `
		DELETE FROM otps
		WHERE email = $1 AND purpose = $2 AND code_hash = $3
		  AND expires_at > CURRENT_TIMESTAMP AND attempts < $4`,
		email, purpose, otpHash(email, purpose, code), OTPMaxAttempts)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}
