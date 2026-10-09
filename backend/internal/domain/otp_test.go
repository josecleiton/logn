package domain

import (
	"context"
	"errors"
	"fmt"
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
	clear := func() {
		conn.Exec(ctx, "DELETE FROM otps WHERE email=$1 AND purpose=$2", email, purpose)
		conn.Exec(ctx, "DELETE FROM email_outbox WHERE kind='otp' AND email=$1", email)
	}
	clear()
	defer clear()

	// Test Save
	if err := repo.SaveOTP(ctx, email, "123456", purpose, "pt-BR", 5*time.Minute); err != nil {
		t.Fatalf("Failed to save OTP: %v", err)
	}

	// O código não fica em claro no banco.
	var stored string
	conn.QueryRow(ctx, "SELECT code_hash FROM otps WHERE email=$1 AND purpose=$2", email, purpose).Scan(&stored)
	if stored == "" || stored == "123456" {
		t.Fatalf("esperava o HMAC do código no banco, veio %q", stored)
	}

	// Pedir outro logo em seguida bate no intervalo mínimo.
	if err := repo.SaveOTP(ctx, email, "654321", purpose, "pt-BR", 5*time.Minute); !errors.Is(err, ErrOTPCooldown) {
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
	if err := repo.SaveOTP(ctx, email, "999999", purpose, "pt-BR", -1*time.Minute); err != nil {
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
	clear := func() {
		conn.Exec(ctx, "DELETE FROM otps WHERE email=$1 AND purpose=$2", email, purpose)
		conn.Exec(ctx, "DELETE FROM email_outbox WHERE kind='otp' AND email=$1", email)
	}
	clear()
	defer clear()

	if err := repo.SaveOTP(ctx, email, "123456", purpose, "pt-BR", 5*time.Minute); err != nil {
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

// Cada reenvio zera as tentativas do código, e um código por minuto dava 7.200 palpites
// por dia contra a mesma conta. As falhas somam através dos reenvios: estourada a
// janela, nenhum código novo sai e nem o certo passa.
func TestOTPFalhasSomamEntreReenvios(t *testing.T) {
	ctx := context.Background()
	repo, conn := otpTestRepo(t)

	email := "test_otp_resend_bruteforce@example.com"
	purpose := OTPPurposeResetPassword
	clear := func() {
		conn.Exec(ctx, "DELETE FROM otps WHERE email=$1 AND purpose=$2", email, purpose)
		conn.Exec(ctx, "DELETE FROM email_outbox WHERE kind='otp' AND email=$1", email)
	}
	clear()
	defer clear()
	// O reenvio de verdade espera OTPResendCooldown; o teste empurra o envio para trás.
	resend := func(code string) error {
		conn.Exec(ctx, "UPDATE otps SET sent_at = sent_at - interval '2 minutes' WHERE email=$1 AND purpose=$2", email, purpose)
		return repo.SaveOTP(ctx, email, code, purpose, "pt-BR", 5*time.Minute)
	}

	if err := repo.SaveOTP(ctx, email, "100000", purpose, "pt-BR", 5*time.Minute); err != nil {
		t.Fatalf("primeiro código: %v", err)
	}
	cycles := OTPMaxFailuresPerWindow / OTPMaxAttempts
	for c := 0; c < cycles; c++ {
		if c > 0 {
			if err := resend("10000" + string(rune('0'+c))); err != nil {
				t.Fatalf("reenvio %d antes do teto: %v", c, err)
			}
		}
		for i := 0; i < OTPMaxAttempts; i++ {
			repo.CheckOTP(ctx, email, "999999", purpose)
		}
	}

	// Janela estourada: o reenvio é recusado, não sai e-mail, e o erro diz quanto falta.
	var locked *OTPLockedError
	if err := resend("123456"); !errors.As(err, &locked) {
		t.Fatalf("reenvio com a janela estourada: esperava *OTPLockedError, veio %v", err)
	}
	if locked.RetryAfter <= OTPFailureWindow-time.Hour || locked.RetryAfter > OTPFailureWindow {
		t.Fatalf("o tempo que falta devia ser quase a janela inteira, veio %v", locked.RetryAfter)
	}

	// Mesmo com tentativas sobrando no código, o certo não passa: quem segura é a janela.
	last := "10000" + string(rune('0'+cycles-1))
	conn.Exec(ctx, "UPDATE otps SET attempts = 0 WHERE email=$1 AND purpose=$2", email, purpose)
	if ok, _ := repo.CheckOTP(ctx, email, last, purpose); ok {
		t.Fatal("código certo passou com a janela de falhas estourada")
	}
	if ok, _ := repo.ConsumeOTP(ctx, email, last, purpose); ok {
		t.Fatal("código consumido com a janela de falhas estourada")
	}

	// A janela vence e tudo volta: código novo sai, e a contagem recomeça do zero.
	conn.Exec(ctx, "UPDATE otps SET failures_since = failures_since - interval '25 hours' WHERE email=$1 AND purpose=$2", email, purpose)
	if err := resend("222222"); err != nil {
		t.Fatalf("reenvio depois da janela: %v", err)
	}
	if ok, _ := repo.CheckOTP(ctx, email, "000000", purpose); ok {
		t.Fatal("código errado conferiu")
	}
	var failures int
	conn.QueryRow(ctx, "SELECT recent_failures FROM otps WHERE email=$1 AND purpose=$2", email, purpose).Scan(&failures)
	if failures != 1 {
		t.Fatalf("a janela vencida devia recomeçar a contagem: veio %d", failures)
	}
	if ok, err := repo.ConsumeOTP(ctx, email, "222222", purpose); err != nil || !ok {
		t.Fatalf("código certo depois da janela: ok=%v err=%v", ok, err)
	}
}

// O limite por IP some trocando de IP, e o intervalo por endereço some trocando de
// endereço. Sem teto global, a rota mandava e-mail a quem quisesse.
func TestOTPTetoGlobalPorHora(t *testing.T) {
	ctx := context.Background()
	repo, conn := otpTestRepo(t)

	email := "test_otp_send_cap@example.com"
	purpose := OTPPurposeVerifyEmail
	clear := func() {
		conn.Exec(ctx, "DELETE FROM otps WHERE email=$1 AND purpose=$2", email, purpose)
		conn.Exec(ctx, "DELETE FROM email_outbox WHERE kind='otp' AND email=$1", email)
	}
	clear()
	defer clear()

	var sent int
	if err := conn.QueryRow(ctx,
		"SELECT count(*) FROM otps WHERE sent_at > CURRENT_TIMESTAMP - interval '1 hour'").Scan(&sent); err != nil {
		t.Fatalf("contando envios: %v", err)
	}
	previous := OTPHourlySendCap
	OTPHourlySendCap = sent
	defer func() { OTPHourlySendCap = previous }()

	if err := repo.SaveOTP(ctx, email, "123456", purpose, "pt-BR", 5*time.Minute); !errors.Is(err, ErrOTPSendCapReached) {
		t.Fatalf("esperava ErrOTPSendCapReached, veio %v", err)
	}
	var rows int
	conn.QueryRow(ctx, "SELECT count(*) FROM otps WHERE email=$1 AND purpose=$2", email, purpose).Scan(&rows)
	if rows != 0 {
		t.Fatal("o teto atingido não deve gravar código")
	}
}

// A purga leva o código que não segura mais nada, e deixa o que ainda conta no teto por
// hora ou ainda trava o e-mail: apagar esses afrouxaria as duas defesas.
func TestOTPPurgeKeepsWhatStillGuards(t *testing.T) {
	ctx := context.Background()
	repo, conn := otpTestRepo(t)

	tag := time.Now().UnixNano()
	email := func(name string) string { return fmt.Sprintf("purge-%s-%d@example.com", name, tag) }
	rows := []struct {
		name                        string
		expires, sent, failingSince string // intervalos a somar a agora; vazio é NULL
		failures                    int
		gone                        bool
	}{
		{"spent", "-2 hours", "-2 hours", "", 0, true},
		{"old-failures", "-2 days", "-2 days", "-2 days", OTPMaxFailuresPerWindow, true},
		{"locked", "-2 hours", "-2 hours", "-1 hour", OTPMaxFailuresPerWindow, false}, // trava de 24 h valendo
		{"counting", "-2 hours", "-2 hours", "-3 hours", 3, false},                    // falhas abaixo do teto, ainda somando
		{"in-cap", "-5 minutes", "-20 minutes", "", 0, false},                         // ainda conta no teto
		{"live", "10 minutes", "0 seconds", "", 0, false},
	}
	for _, r := range rows {
		t.Cleanup(func() { conn.Exec(ctx, `DELETE FROM otps WHERE email = $1`, email(r.name)) })
		if _, err := conn.Exec(ctx, `
			INSERT INTO otps (email, purpose, code_hash, expires_at, attempts, sent_at, recent_failures, failures_since)
			VALUES ($1, 'verify_email', 'x', CURRENT_TIMESTAMP + $2::interval, 0, CURRENT_TIMESTAMP + $3::interval,
			        $5, CURRENT_TIMESTAMP + NULLIF($4, '')::interval)`,
			email(r.name), r.expires, r.sent, r.failingSince, r.failures); err != nil {
			t.Fatalf("%s: %v", r.name, err)
		}
	}

	if _, err := repo.PurgeStaleOTPs(ctx); err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		var n int
		conn.QueryRow(ctx, `SELECT count(*) FROM otps WHERE email = $1`, email(r.name)).Scan(&n)
		if r.gone && n != 0 {
			t.Errorf("%s devia ter saído", r.name)
		}
		if !r.gone && n != 1 {
			t.Errorf("%s não podia sair", r.name)
		}
	}
}
