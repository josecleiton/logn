package domain

import (
	"context"
	"fmt"
	"testing"
)

func clearLoginAttempts(t *testing.T, repo *Repository, emails ...string) {
	t.Helper()
	clear := func() {
		for _, e := range emails {
			repo.ClearLoginAttempts(context.Background(), e)
		}
	}
	clear()
	t.Cleanup(clear)
}

// Do mesmo IP, passadas LoginMaxAttemptsPerSource tentativas na janela, a seguinte é
// recusada antes do Argon2. De outro IP a conta continua entrando: travar a conta de
// alguém daqui não trava a pessoa.
func TestLoginTravaPorIPSemTravarOsOutros(t *testing.T) {
	ctx := context.Background()
	repo, conn := otpTestRepo(t)
	email := "test_login_lockout@example.com"
	clearLoginAttempts(t, repo, email)

	for i := 1; i <= LoginMaxAttemptsPerSource; i++ {
		if ok, err := repo.NoteLoginAttempt(ctx, email, "203.0.113.7"); err != nil || !ok {
			t.Fatalf("tentativa %d devia passar: ok=%v err=%v", i, ok, err)
		}
	}
	if ok, _ := repo.NoteLoginAttempt(ctx, email, "203.0.113.7"); ok {
		t.Fatalf("a tentativa %d do mesmo IP passou do teto", LoginMaxAttemptsPerSource+1)
	}
	if ok, err := repo.NoteLoginAttempt(ctx, email, "198.51.100.9"); err != nil || !ok {
		t.Fatalf("outro IP devia passar: ok=%v err=%v", ok, err)
	}

	// Nem o e-mail nem o IP ficam guardados: só os HMAC.
	var plain int
	conn.QueryRow(ctx, "SELECT count(*) FROM login_failures WHERE email_hmac = $1 OR scope = $2 OR scope = $3",
		email, "203.0.113.7", "198.51.100.9").Scan(&plain)
	if plain != 0 {
		t.Fatal("e-mail ou IP em claro na tabela")
	}

	// A janela vence e a contagem recomeça.
	conn.Exec(ctx, "UPDATE login_failures SET window_start = window_start - interval '16 minutes' WHERE email_hmac=$1", loginKey(email))
	if ok, err := repo.NoteLoginAttempt(ctx, email, "203.0.113.7"); err != nil || !ok {
		t.Fatalf("depois da janela devia passar: ok=%v err=%v", ok, err)
	}
}

// Somando todos os IPs, passadas LoginMaxAttempts tentativas, nenhum IP passa: é o que
// segura o chute distribuído.
func TestLoginTravaOChuteDistribuido(t *testing.T) {
	ctx := context.Background()
	repo, _ := otpTestRepo(t)
	email := "test_login_distributed@example.com"
	clearLoginAttempts(t, repo, email)

	for i := 1; i <= LoginMaxAttempts; i++ {
		if ok, err := repo.NoteLoginAttempt(ctx, email, fmt.Sprintf("2001:db8:%x::/64", i)); err != nil || !ok {
			t.Fatalf("tentativa %d de IP novo devia passar: ok=%v err=%v", i, ok, err)
		}
	}
	if ok, _ := repo.NoteLoginAttempt(ctx, email, "2001:db8:ffff::/64"); ok {
		t.Fatalf("a tentativa %d somando os IPs passou do teto", LoginMaxAttempts+1)
	}

	// O login certo, ou a troca de senha pelo código, zera tudo, de todo IP.
	if err := repo.ClearLoginAttempts(ctx, email); err != nil {
		t.Fatalf("zerando: %v", err)
	}
	if ok, err := repo.NoteLoginAttempt(ctx, email, "2001:db8:ffff::/64"); err != nil || !ok {
		t.Fatalf("depois de zerar devia passar: ok=%v err=%v", ok, err)
	}
}

// A tentativa que o balde do IP recusou não conta na soma: senão um IP sozinho levava a
// soma ao teto e travava a conta para todo IP.
func TestLoginRecusadaPeloIPNaoContaNaSoma(t *testing.T) {
	ctx := context.Background()
	repo, conn := otpTestRepo(t)
	email := "test_login_one_source@example.com"
	clearLoginAttempts(t, repo, email)

	for i := 0; i < LoginMaxAttempts+5; i++ {
		repo.NoteLoginAttempt(ctx, email, "203.0.113.7")
	}
	var total int
	conn.QueryRow(ctx, "SELECT failures FROM login_failures WHERE email_hmac = $1 AND scope = ''", loginKey(email)).Scan(&total)
	if total != LoginMaxAttemptsPerSource {
		t.Fatalf("a soma devia parar em %d com um IP só, foi a %d", LoginMaxAttemptsPerSource, total)
	}
	if ok, err := repo.NoteLoginAttempt(ctx, email, "198.51.100.9"); err != nil || !ok {
		t.Fatalf("outro IP devia passar: ok=%v err=%v", ok, err)
	}
}

// A purga diária leva as contagens de janela vencida e deixa as vivas.
func TestPurgeLoginAttemptsLevaSoAsVencidas(t *testing.T) {
	ctx := context.Background()
	repo, conn := otpTestRepo(t)
	old, live := "test_login_purge_old@example.com", "test_login_purge_live@example.com"
	clearLoginAttempts(t, repo, old, live)

	repo.NoteLoginAttempt(ctx, old, "203.0.113.7")
	repo.NoteLoginAttempt(ctx, live, "203.0.113.7")
	conn.Exec(ctx, "UPDATE login_failures SET window_start = window_start - interval '1 hour' WHERE email_hmac=$1", loginKey(old))

	if _, err := repo.PurgeLoginAttempts(ctx); err != nil {
		t.Fatalf("purga: %v", err)
	}
	var oldLeft, liveLeft int
	conn.QueryRow(ctx, "SELECT count(*) FROM login_failures WHERE email_hmac = $1", loginKey(old)).Scan(&oldLeft)
	conn.QueryRow(ctx, "SELECT count(*) FROM login_failures WHERE email_hmac = $1", loginKey(live)).Scan(&liveLeft)
	if oldLeft != 0 || liveLeft != 2 {
		t.Fatalf("esperava só as contagens vivas (2), ficaram vencidas=%d vivas=%d", oldLeft, liveLeft)
	}
}
