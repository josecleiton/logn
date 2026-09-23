package domain

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestGenerateOTP(t *testing.T) {
	otp := GenerateOTP()
	if len(otp) != 6 {
		t.Errorf("Expected OTP length 6, got %v", len(otp))
	}

	for _, c := range otp {
		if c < '0' || c > '9' {
			t.Errorf("Expected OTP to contain only numbers, got %v", otp)
			break
		}
	}
}

func otpTestRepo(t *testing.T) (*Repository, *pgxpool.Pool) {
	t.Helper()
	conn, err := pgxpool.New(context.Background(), "postgres://logn_user:logn_password@localhost:5432/logn_db?sslmode=disable")
	if err != nil {
		t.Skip("Database not available for integration test")
	}
	t.Cleanup(conn.Close)
	return NewRepository(conn), conn
}

// Ensure the db functions work correctly when the DB is injected.
func TestOTPRepository(t *testing.T) {
	ctx := context.Background()
	repo, conn := otpTestRepo(t)

	email := "test_otp@example.com"
	purpose := OTPPurposeVerifyEmail
	clear := func() { conn.Exec(ctx, "DELETE FROM otps WHERE email=$1 AND purpose=$2", email, purpose) }
	clear()
	defer clear()

	// Test Save
	if err := repo.SaveOTP(ctx, email, "123456", purpose, 5*time.Minute); err != nil {
		t.Fatalf("Failed to save OTP: %v", err)
	}

	// O código não fica em claro no banco.
	var stored string
	conn.QueryRow(ctx, "SELECT code_hash FROM otps WHERE email=$1 AND purpose=$2", email, purpose).Scan(&stored)
	if stored == "" || stored == "123456" {
		t.Fatalf("esperava o HMAC do código no banco, veio %q", stored)
	}

	// Pedir outro logo em seguida bate no intervalo mínimo.
	if err := repo.SaveOTP(ctx, email, "654321", purpose, 5*time.Minute); !errors.Is(err, ErrOTPCooldown) {
		t.Fatalf("esperava ErrOTPCooldown, veio %v", err)
	}

	// Test Verify success
	valid, err := repo.ConsumeOTP(ctx, email, "123456", purpose)
	if err != nil || !valid {
		t.Errorf("Expected valid OTP, got false or err: %v", err)
	}

	// Test Single-use (should be deleted after first verification)
	valid, _ = repo.ConsumeOTP(ctx, email, "123456", purpose)
	if valid {
		t.Errorf("Expected OTP to be invalid after first use")
	}

	// Test Expiry behavior (indirectly tested by injecting an expired time in Save)
	clear()
	if err := repo.SaveOTP(ctx, email, "999999", purpose, -1*time.Minute); err != nil {
		t.Errorf("Failed to save expired OTP: %v", err)
	}

	valid, _ = repo.ConsumeOTP(ctx, email, "999999", purpose)
	if valid {
		t.Errorf("Expected expired OTP to be invalid")
	}
}

// Sem limite de tentativas, seis dígitos se varriam pelo verify-otp dentro do prazo e
// davam a troca de senha de qualquer conta.
func TestOTPMorreDepoisDeTentativasErradas(t *testing.T) {
	ctx := context.Background()
	repo, conn := otpTestRepo(t)

	email := "test_otp_bruteforce@example.com"
	purpose := OTPPurposeResetPassword
	clear := func() { conn.Exec(ctx, "DELETE FROM otps WHERE email=$1 AND purpose=$2", email, purpose) }
	clear()
	defer clear()

	if err := repo.SaveOTP(ctx, email, "123456", purpose, 5*time.Minute); err != nil {
		t.Fatalf("Failed to save OTP: %v", err)
	}

	// Acertar não conta tentativa: o registro confere no verify-otp e gasta depois.
	if ok, err := repo.CheckOTP(ctx, email, "123456", purpose); err != nil || !ok {
		t.Fatalf("código certo devia conferir: ok=%v err=%v", ok, err)
	}

	for i := 0; i < OTPMaxAttempts; i++ {
		if ok, _ := repo.CheckOTP(ctx, email, "000000", purpose); ok {
			t.Fatalf("código errado conferiu na tentativa %d", i+1)
		}
	}

	// Esgotado, nem o código certo passa mais.
	if ok, _ := repo.CheckOTP(ctx, email, "123456", purpose); ok {
		t.Fatal("código certo passou depois de esgotar as tentativas")
	}
	if ok, _ := repo.ConsumeOTP(ctx, email, "123456", purpose); ok {
		t.Fatal("código esgotado foi consumido")
	}
}
