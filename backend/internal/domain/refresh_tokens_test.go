package domain

import (
	"context"
	"errors"
	"testing"
	"time"
)

// A purga leva só o vencido há mais de um dia. O revogado ainda no prazo fica, e o
// reuso dele continua derrubando as outras sessões da conta.
func TestPurgeKeepsTheRevokedTokenThatDetectsReuse(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()

	id, _ := newDeletionTestUser(t, repo) // já vem com "tok-<id>", vivo por uma hora
	old, recent, live := "old-"+id, "recent-"+id, "tok-"+id
	if err := repo.CreateRefreshToken(ctx, id, old, time.Now().Add(-2*RefreshTokenPurgeGrace)); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateRefreshToken(ctx, id, recent, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}

	// Rotaciona o vivo: ele fica revogado e no prazo, e sai um novo.
	if err := repo.RotateRefreshToken(ctx, live, id, "next-"+id, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	if _, err := repo.PurgeExpiredRefreshTokens(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetRefreshToken(ctx, old); err == nil {
		t.Error("o vencido há dois dias devia ter saído")
	}
	for _, kept := range []string{recent, live, "next-" + id} {
		if _, err := repo.GetRefreshToken(ctx, kept); err != nil {
			t.Errorf("%s não devia sair na purga: %v", kept, err)
		}
	}

	// O revogado reapresentado é reuso: derruba a sessão nova, na mesma transação.
	err := repo.RotateRefreshToken(ctx, live, id, "again-"+id, time.Now().Add(time.Hour))
	if !errors.Is(err, ErrRefreshTokenAlreadyUsed) {
		t.Fatalf("reuso devia dar ErrRefreshTokenAlreadyUsed, veio %v", err)
	}
	next, err := repo.GetRefreshToken(ctx, "next-"+id)
	if err != nil {
		t.Fatal(err)
	}
	if !next.Revoked {
		t.Error("o reuso não revogou a sessão que estava viva")
	}
	if _, err := repo.GetRefreshToken(ctx, "again-"+id); err == nil {
		t.Error("o reuso emitiu token novo")
	}
}

// Dois tokens com o mesmo hash seriam o mesmo token.
func TestRefreshTokenHashIsUnique(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)

	id, _ := newDeletionTestUser(t, repo)
	if err := repo.CreateRefreshToken(context.Background(), id, "tok-"+id, time.Now().Add(time.Hour)); err == nil {
		t.Fatal("o banco aceitou dois refresh tokens com o mesmo hash")
	}
}
