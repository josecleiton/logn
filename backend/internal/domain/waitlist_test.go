package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestWaitlistToken(t *testing.T) {
	id := "0b8e7c9a-1f2d-4e3b-9a8c-7d6e5f4a3b2c"
	tok := WaitlistToken(id, WaitlistActionConfirm)
	if got, ok := ParseWaitlistToken(tok, WaitlistActionConfirm); !ok || got != id {
		t.Fatalf("ParseWaitlistToken(%q) = %q, %v", tok, got, ok)
	}
	// O link de confirmação não tira ninguém da lista, e vice-versa.
	if _, ok := ParseWaitlistToken(tok, WaitlistActionLeave); ok {
		t.Error("token de confirmação aceito na saída")
	}
	other := "1b8e7c9a-1f2d-4e3b-9a8c-7d6e5f4a3b2c"
	_, mac, _ := strings.Cut(tok, ".")
	for _, bad := range []string{
		"",
		id,
		id + ".",
		other + "." + mac, // assinatura de um id servindo para outro
		id + "." + strings.Repeat("0", 64),
		strings.ToUpper(id) + "." + mac,
		"../../x." + mac,
	} {
		if _, ok := ParseWaitlistToken(bad, WaitlistActionConfirm); ok {
			t.Errorf("token %q aceito", bad)
		}
	}
}

func newWaitlistEmail(t *testing.T, repo *Repository) string {
	t.Helper()
	addr := fmt.Sprintf("waitlist-%d@example.com", time.Now().UnixNano())
	t.Cleanup(func() {
		repo.db.Exec(context.Background(), `DELETE FROM waitlist_entries WHERE email = $1`, addr)
	})
	return addr
}

func TestJoinWaitlist(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()
	addr := newWaitlistEmail(t, repo)

	id, send, err := repo.JoinWaitlist(ctx, addr, "en")
	if err != nil || !send || id == "" {
		t.Fatalf("primeira inscrição: id=%q send=%v err=%v", id, send, err)
	}

	// A pendente recebe um e-mail só: pedir de novo, no mesmo dia ou dias depois, não
	// reenvia nem muda nada.
	for _, age := range []string{"0", "2 days"} {
		repo.db.Exec(ctx, `UPDATE waitlist_entries SET sent_at = sent_at - $2::interval WHERE id = $1`, id, age)
		if again, send, err := repo.JoinWaitlist(ctx, addr, "es"); err != nil || send || again != "" {
			t.Fatalf("reinscrição com envio de %s: id=%q send=%v err=%v", age, again, send, err)
		}
	}
	if e, _ := repo.GetWaitlistEntry(ctx, id); e.Locale != "en" {
		t.Errorf("a reinscrição trocou a língua para %q", e.Locale)
	}

	// O envio falhou: o próximo pedido tenta de novo, na língua nova.
	if err := repo.ReleaseWaitlistSend(ctx, id); err != nil {
		t.Fatal(err)
	}
	if again, send, err := repo.JoinWaitlist(ctx, addr, "es"); err != nil || !send || again != id {
		t.Fatalf("reinscrição depois de liberar: id=%q send=%v err=%v", again, send, err)
	}

	// Confirmada não recebe confirmação de novo, nem com o envio liberado.
	if _, err := repo.ConfirmWaitlist(ctx, id); err != nil {
		t.Fatal(err)
	}
	repo.ReleaseWaitlistSend(ctx, id)
	if _, send, err := repo.JoinWaitlist(ctx, addr, "pt-BR"); err != nil || send {
		t.Fatalf("inscrição confirmada mandou de novo: send=%v err=%v", send, err)
	}
}

// Quem volta depois de a pendente vencer se inscreve de novo, e só aí: é no máximo um
// e-mail a cada WaitlistPendingTTL por endereço.
func TestJoinWaitlistAfterExpiry(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()
	addr := newWaitlistEmail(t, repo)

	first, _, _ := repo.JoinWaitlist(ctx, addr, "en")
	repo.db.Exec(ctx, `UPDATE waitlist_entries SET created_at = created_at - interval '6 days' WHERE id = $1`, first)
	if _, send, _ := repo.JoinWaitlist(ctx, addr, "en"); send {
		t.Fatal("pendente no sexto dia recebeu outro e-mail")
	}
	repo.db.Exec(ctx, `UPDATE waitlist_entries SET created_at = created_at - interval '2 days' WHERE id = $1`, first)
	again, send, err := repo.JoinWaitlist(ctx, addr, "es")
	if err != nil || !send || again == first {
		t.Fatalf("depois de vencer: id=%q (antes %q) send=%v err=%v", again, first, send, err)
	}
}

func TestJoinWaitlistHourlyCap(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()

	var sent int
	repo.db.QueryRow(ctx, `SELECT count(*) FROM waitlist_entries WHERE sent_at > CURRENT_TIMESTAMP - interval '1 hour'`).Scan(&sent)
	saved := WaitlistHourlySendCap
	WaitlistHourlySendCap = sent + 1
	t.Cleanup(func() { WaitlistHourlySendCap = saved })

	if _, send, err := repo.JoinWaitlist(ctx, newWaitlistEmail(t, repo), "en"); err != nil || !send {
		t.Fatalf("abaixo do teto: send=%v err=%v", send, err)
	}
	blocked := newWaitlistEmail(t, repo)
	if _, _, err := repo.JoinWaitlist(ctx, blocked, "en"); !errors.Is(err, ErrWaitlistBusy) {
		t.Fatalf("no teto: %v", err)
	}
	var n int
	repo.db.QueryRow(ctx, `SELECT count(*) FROM waitlist_entries WHERE email = $1`, blocked).Scan(&n)
	if n != 0 {
		t.Error("o pedido acima do teto gravou a inscrição")
	}
}

func TestConfirmAndLeaveWaitlist(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()
	addr := newWaitlistEmail(t, repo)

	id, _, err := repo.JoinWaitlist(ctx, addr, "es")
	if err != nil {
		t.Fatal(err)
	}
	e, err := repo.GetWaitlistEntry(ctx, id)
	if err != nil || e.Confirmed || e.Locale != "es" {
		t.Fatalf("pendente: %+v err=%v", e, err)
	}

	lang, err := repo.ConfirmWaitlist(ctx, id)
	if err != nil || lang != "es" {
		t.Fatalf("ConfirmWaitlist: %q %v", lang, err)
	}
	var first time.Time
	repo.db.QueryRow(ctx, `SELECT confirmed_at FROM waitlist_entries WHERE id = $1`, id).Scan(&first)
	if _, err := repo.ConfirmWaitlist(ctx, id); err != nil {
		t.Fatalf("confirmar de novo: %v", err)
	}
	var second time.Time
	repo.db.QueryRow(ctx, `SELECT confirmed_at FROM waitlist_entries WHERE id = $1`, id).Scan(&second)
	if !first.Equal(second) {
		t.Errorf("confirmar de novo mudou a data: %v → %v", first, second)
	}

	if lang, err := repo.LeaveWaitlist(ctx, id); err != nil || lang != "es" {
		t.Fatalf("LeaveWaitlist: %q %v", lang, err)
	}
	if _, err := repo.GetWaitlistEntry(ctx, id); !errors.Is(err, ErrWaitlistNotFound) {
		t.Errorf("a saída não apagou a inscrição: %v", err)
	}
	if _, err := repo.LeaveWaitlist(ctx, id); !errors.Is(err, ErrWaitlistNotFound) {
		t.Errorf("sair de novo: %v", err)
	}
	if _, err := repo.ConfirmWaitlist(ctx, id); !errors.Is(err, ErrWaitlistNotFound) {
		t.Errorf("confirmar depois de sair: %v", err)
	}
}

func TestPendingWaitlistExpires(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()

	stale := newWaitlistEmail(t, repo)
	staleID, _, _ := repo.JoinWaitlist(ctx, stale, "pt-BR")
	fresh := newWaitlistEmail(t, repo)
	freshID, _, _ := repo.JoinWaitlist(ctx, fresh, "pt-BR")
	confirmed := newWaitlistEmail(t, repo)
	confirmedID, _, _ := repo.JoinWaitlist(ctx, confirmed, "pt-BR")
	repo.ConfirmWaitlist(ctx, confirmedID)

	repo.db.Exec(ctx, `UPDATE waitlist_entries SET created_at = created_at - interval '8 days' WHERE id = ANY($1)`,
		[]string{staleID, confirmedID})

	// Vencida e ainda não purgada: o link já não vale.
	if _, err := repo.GetWaitlistEntry(ctx, staleID); !errors.Is(err, ErrWaitlistNotFound) {
		t.Errorf("pendente vencida ainda lida: %v", err)
	}
	if _, err := repo.ConfirmWaitlist(ctx, staleID); !errors.Is(err, ErrWaitlistNotFound) {
		t.Errorf("pendente vencida confirmou: %v", err)
	}

	if _, err := repo.PurgePendingWaitlist(ctx); err != nil {
		t.Fatal(err)
	}
	var left []string
	rows, _ := repo.db.Query(ctx, `SELECT id::text FROM waitlist_entries WHERE id = ANY($1) ORDER BY id`,
		[]string{staleID, freshID, confirmedID})
	for rows.Next() {
		var id string
		rows.Scan(&id)
		left = append(left, id)
	}
	rows.Close()
	if len(left) != 2 || strings.Contains(strings.Join(left, ","), staleID) {
		t.Errorf("a purga deixou %v; esperava só a nova e a confirmada", left)
	}
}
