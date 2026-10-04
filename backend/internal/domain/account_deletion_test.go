package domain

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// newDeletionTestUser cria uma conta com sessão, progresso, cadeia de sync, aceite e
// código pendente — tudo o que o expurgo tem de levar junto — e a apaga no fim.
func newDeletionTestUser(t *testing.T, repo *Repository) (userID, email string) {
	t.Helper()
	ctx := context.Background()
	email = fmt.Sprintf("deletion-%d@logn.test", time.Now().UnixNano())

	id, err := repo.CreateUser(ctx, email, "hash", true, "BR", []LegalAcceptance{{Kind: "terms", Version: 1, Locale: "pt-BR"}}, ClientInfo{}, "pt-BR")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := repo.CreateRefreshToken(ctx, id, "tok-"+id, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("CreateRefreshToken: %v", err)
	}
	for _, q := range []string{
		`INSERT INTO user_sync_state (user_id, last_hash) VALUES ($1, 'h')`,
		`INSERT INTO game_events (id, user_id, event_type, payload_json, timestamp, previous_hash, current_hash)
		 VALUES ('e1', $1, 'MATCH_ANSWER', '{}', 1, 'p', 'h')`,
	} {
		if _, err := repo.db.Exec(ctx, q, id); err != nil {
			t.Fatalf("semear %q: %v", q, err)
		}
	}
	if err := repo.SaveOTP(ctx, email, "123456", OTPPurposeResetPassword, "pt-BR", time.Minute); err != nil {
		t.Fatalf("SaveOTP: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		if _, err := repo.db.Exec(ctx, `DELETE FROM users WHERE id = $1`, id); err != nil {
			t.Errorf("conta de teste %s ficou no banco: %v", email, err)
		}
		repo.db.Exec(ctx, `DELETE FROM game_events WHERE user_id = $1`, id)
		repo.db.Exec(ctx, `DELETE FROM user_sync_state WHERE user_id = $1`, id)
		repo.db.Exec(ctx, `DELETE FROM otps WHERE email = $1`, email)
		repo.db.Exec(ctx, `DELETE FROM email_outbox WHERE kind = 'otp' AND email = $1`, email)
	})
	return id, email
}

func countRows(t *testing.T, repo *Repository, query string, arg string) int {
	t.Helper()
	var n int
	if err := repo.db.QueryRow(context.Background(), query, arg).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func TestCreateUserStoresCountry(t *testing.T) {
	conn := setupTestDB(t)
	// Cleanup, não defer: o defer fecharia o pool antes da limpeza de
	// newDeletionTestUser, e as contas de teste ficavam no banco.
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)

	id, _ := newDeletionTestUser(t, repo)
	var country string
	if err := conn.QueryRow(context.Background(), `SELECT country FROM users WHERE id = $1`, id).Scan(&country); err != nil {
		t.Fatal(err)
	}
	if country != "BR" {
		t.Fatalf("country = %q", country)
	}
}

// Pedir a exclusão desativa a conta e derruba todas as sessões. Antes só gravava a
// data, e o refresh token de outro aparelho seguia valendo.
func TestMarkAccountForDeletionRevokesSessions(t *testing.T) {
	conn := setupTestDB(t)
	// Cleanup, não defer: o defer fecharia o pool antes da limpeza de
	// newDeletionTestUser, e as contas de teste ficavam no banco.
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()

	id, _ := newDeletionTestUser(t, repo)
	before := time.Now()
	purgeAfter, err := repo.MarkAccountForDeletion(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if purgeAfter.Before(before.Add(AccountDeletionGrace - time.Minute)) {
		t.Errorf("purge_after = %v, esperava uns 30 dias à frente", purgeAfter)
	}
	if repo.IsUserActive(ctx, id) {
		t.Error("conta segue ativa")
	}
	if n := countRows(t, repo, `SELECT count(*) FROM refresh_tokens WHERE user_id = $1 AND revoked = FALSE`, id); n != 0 {
		t.Errorf("%d sessões seguem vivas", n)
	}

	// Pedir de novo não empurra o prazo.
	again, err := repo.MarkAccountForDeletion(ctx, id)
	if err != nil || !again.Equal(purgeAfter) {
		t.Errorf("segundo pedido mudou o prazo: %v → %v (%v)", purgeAfter, again, err)
	}
}

// O login dentro da carência reativa a conta, e só diz que reativou quando havia o que
// reativar.
func TestCancelAccountDeletion(t *testing.T) {
	conn := setupTestDB(t)
	// Cleanup, não defer: o defer fecharia o pool antes da limpeza de
	// newDeletionTestUser, e as contas de teste ficavam no banco.
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()

	id, _ := newDeletionTestUser(t, repo)
	if restored, err := repo.CancelAccountDeletion(ctx, id); err != nil || restored {
		t.Fatalf("conta ativa não tem o que cancelar: %v %v", restored, err)
	}
	repo.MarkAccountForDeletion(ctx, id)
	if restored, err := repo.CancelAccountDeletion(ctx, id); err != nil || !restored {
		t.Fatalf("devia cancelar: %v %v", restored, err)
	}
	if !repo.IsUserActive(ctx, id) {
		t.Error("conta não voltou")
	}
}

// Redefinir a senha também cancela: é o caminho de quem esqueceu a senha e quer a
// conta de volta.
func TestResetPasswordCancelsDeletion(t *testing.T) {
	conn := setupTestDB(t)
	// Cleanup, não defer: o defer fecharia o pool antes da limpeza de
	// newDeletionTestUser, e as contas de teste ficavam no banco.
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()

	id, email := newDeletionTestUser(t, repo)
	repo.MarkAccountForDeletion(ctx, id)

	got, restored, err := repo.ResetUserPassword(ctx, email, "novo-hash")
	if err != nil || got != id || !restored {
		t.Fatalf("got=%s restored=%v err=%v", got, restored, err)
	}
	if !repo.IsUserActive(ctx, id) {
		t.Error("conta não voltou")
	}
	if _, restored, _ := repo.ResetUserPassword(ctx, email, "outro-hash"); restored {
		t.Error("segunda troca não tinha exclusão a cancelar")
	}
}

// O expurgo leva a conta e tudo o que é dela, inclusive os códigos por e-mail, e não
// toca em conta dentro da carência.
func TestPurgeDeletedAccounts(t *testing.T) {
	conn := setupTestDB(t)
	// Cleanup, não defer: o defer fecharia o pool antes da limpeza de
	// newDeletionTestUser, e as contas de teste ficavam no banco.
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()

	expired, expiredEmail := newDeletionTestUser(t, repo)
	recent, _ := newDeletionTestUser(t, repo)
	repo.MarkAccountForDeletion(ctx, expired)
	repo.MarkAccountForDeletion(ctx, recent)
	if _, err := conn.Exec(ctx,
		`UPDATE users SET deletion_requested_at = now() - interval '31 days' WHERE id = $1`, expired); err != nil {
		t.Fatal(err)
	}

	if _, err := repo.PurgeDeletedAccounts(ctx); err != nil {
		t.Fatal(err)
	}

	for query, arg := range map[string]string{
		`SELECT count(*) FROM users WHERE id = $1`:                      expired,
		`SELECT count(*) FROM refresh_tokens WHERE user_id = $1`:        expired,
		`SELECT count(*) FROM legal_acceptances WHERE user_id = $1`:     expired,
		`SELECT count(*) FROM game_events WHERE user_id = $1::text`:     expired,
		`SELECT count(*) FROM user_sync_state WHERE user_id = $1::text`: expired,
		`SELECT count(*) FROM otps WHERE email = $1`:                    expiredEmail,
		`SELECT count(*) FROM email_outbox WHERE user_id = $1`:          expired,
		`SELECT count(*) FROM email_outbox WHERE email = $1`:            expiredEmail,
	} {
		if n := countRows(t, repo, query, arg); n != 0 {
			t.Errorf("%s: sobraram %d linhas", query, n)
		}
	}
	if n := countRows(t, repo, `SELECT count(*) FROM users WHERE id = $1`, recent); n != 1 {
		t.Error("conta dentro da carência foi apagada")
	}

	// Rodar de novo não acha a conta que já saiu.
	if _, err := repo.PurgeDeletedAccounts(ctx); err != nil {
		t.Fatal(err)
	}
}
