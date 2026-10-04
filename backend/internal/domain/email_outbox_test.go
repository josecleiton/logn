package domain

import (
	"bytes"
	"context"
	"errors"
	"os"
	"slices"
	"testing"
	"time"
)

// O código da caixa de saída é cifrado com uma chave derivada de TRACK_KEY_SECRET, e
// sem ela o SaveOTP falha. Os testes do pacote rodam com a chave de teste.
func TestMain(m *testing.M) {
	TrackKeySecret = []byte("test-track-key-secret-32-bytes!!")
	os.Exit(m.Run())
}

// outboxRow é o que os testes conferem numa linha de `email_outbox`.
type outboxRow struct {
	status   string
	attempts int
	sealed   []byte
	lastErr  *string
}

func readOutbox(t *testing.T, repo *Repository, id string) outboxRow {
	t.Helper()
	var r outboxRow
	if err := repo.db.QueryRow(context.Background(), `
		SELECT status, attempts, code_ciphertext, last_error FROM email_outbox WHERE id = $1`,
		id).Scan(&r.status, &r.attempts, &r.sealed, &r.lastErr); err != nil {
		t.Fatalf("linha %s: %v", id, err)
	}
	return r
}

// newOTPOutbox grava um código pelo SaveOTP e devolve o id do e-mail dele, anotado no
// OutboxTracker como num pedido de verdade.
func newOTPOutbox(t *testing.T, repo *Repository, email, code string) string {
	t.Helper()
	ctx, tracker := WithOutboxTracker(context.Background())
	if err := repo.SaveOTP(ctx, email, code, OTPPurposeVerifyEmail, "en", OTPValidity); err != nil {
		t.Fatalf("SaveOTP: %v", err)
	}
	ids := tracker.Drain()
	if len(ids) != 1 {
		t.Fatalf("SaveOTP anotou %d e-mails, want 1", len(ids))
	}
	return ids[0]
}

func outboxTestEmail(t *testing.T, repo *Repository) string {
	t.Helper()
	email := "outbox-" + testUUID(t)[:12] + "@example.com"
	t.Cleanup(func() {
		ctx := context.Background()
		repo.db.Exec(ctx, `DELETE FROM otps WHERE email = $1`, email)
		repo.db.Exec(ctx, `DELETE FROM email_outbox WHERE kind = 'otp' AND email = $1`, email)
	})
	return email
}

// O código vai cifrado para a linha, sai aberto só para quem envia, e some da linha
// assim que ela é enviada.
func TestOutboxOTPIsSealedAndWiped(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	email := outboxTestEmail(t, repo)

	id := newOTPOutbox(t, repo, email, "424242")
	row := readOutbox(t, repo, id)
	if row.status != "pending" || len(row.sealed) == 0 {
		t.Fatalf("linha nova: %+v", row)
	}
	if bytes.Contains(row.sealed, []byte("424242")) {
		t.Fatal("o código está em claro na caixa de saída")
	}

	var got OutboxMessage
	outcome, err := repo.DeliverOutbox(context.Background(), id, false, func(m OutboxMessage) error {
		got = m
		return nil
	})
	if err != nil || outcome != OutboxSent {
		t.Fatalf("entrega: %v %v", outcome, err)
	}
	if got.Code != "424242" || got.Email != email || got.Purpose != OTPPurposeVerifyEmail || got.Locale != "en" {
		t.Fatalf("e-mail montado: %+v", got)
	}
	row = readOutbox(t, repo, id)
	if row.status != "sent" || row.sealed != nil || row.attempts != 1 {
		t.Fatalf("depois de enviar: %+v", row)
	}

	// A fila entrega a mesma tarefa de novo: nada sai.
	outcome, err = repo.DeliverOutbox(context.Background(), id, false, func(OutboxMessage) error {
		t.Fatal("e-mail enviado duas vezes")
		return nil
	})
	if err != nil || outcome != OutboxSkipped {
		t.Fatalf("segunda entrega: %v %v", outcome, err)
	}
}

// O código cifrado de uma linha não abre em outra: copiar o texto cifrado para outro
// endereço não manda o código para lá.
func TestOutboxOTPCiphertextIsBoundToTheRecipient(t *testing.T) {
	sealed, err := sealOutboxCode("a@example.com", OTPPurposeVerifyEmail, "123456")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := openOutboxCode("b@example.com", OTPPurposeVerifyEmail, sealed); err == nil {
		t.Fatal("o código abriu com outro endereço")
	}
	if _, err := openOutboxCode("a@example.com", OTPPurposeResetPassword, sealed); err == nil {
		t.Fatal("o código abriu com outro propósito")
	}
}

// Pedido outro código, o antigo não sai mais: só confundiria quem o recebesse.
func TestOutboxSkipsASupersededOTP(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()
	email := outboxTestEmail(t, repo)

	old := newOTPOutbox(t, repo, email, "111111")
	// Passa o intervalo de reenvio.
	conn.Exec(ctx, `UPDATE otps SET sent_at = sent_at - interval '2 minutes' WHERE email = $1`, email)
	fresh := newOTPOutbox(t, repo, email, "222222")

	outcome, err := repo.DeliverOutbox(ctx, old, false, func(OutboxMessage) error {
		t.Fatal("o código antigo saiu")
		return nil
	})
	if err != nil || outcome != OutboxSkipped {
		t.Fatalf("código antigo: %v %v", outcome, err)
	}
	if row := readOutbox(t, repo, old); row.status != "skipped" || row.sealed != nil {
		t.Fatalf("código antigo depois: %+v", row)
	}

	var sent string
	if _, err := repo.DeliverOutbox(ctx, fresh, false, func(m OutboxMessage) error {
		sent = m.Code
		return nil
	}); err != nil || sent != "222222" {
		t.Fatalf("código novo: %q %v", sent, err)
	}
}

// Código já usado também não sai.
func TestOutboxSkipsAConsumedOTP(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()
	email := outboxTestEmail(t, repo)

	id := newOTPOutbox(t, repo, email, "333333")
	if ok, err := repo.ConsumeOTP(ctx, email, "333333", OTPPurposeVerifyEmail); err != nil || !ok {
		t.Fatalf("consumo: %v %v", ok, err)
	}
	outcome, err := repo.DeliverOutbox(ctx, id, false, func(OutboxMessage) error {
		t.Fatal("código usado saiu")
		return nil
	})
	if err != nil || outcome != OutboxSkipped {
		t.Fatalf("%v %v", outcome, err)
	}
}

// Falha antes da última tentativa conta e deixa a linha pendente, com o código; na
// última, a linha desiste e o código some.
func TestOutboxRetriesThenGivesUp(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()
	email := outboxTestEmail(t, repo)
	id := newOTPOutbox(t, repo, email, "555555")
	fail := func(OutboxMessage) error { return errors.New("421 try again later") }

	outcome, err := repo.DeliverOutbox(ctx, id, false, fail)
	if err != nil || outcome != OutboxRetry {
		t.Fatalf("primeira falha: %v %v", outcome, err)
	}
	row := readOutbox(t, repo, id)
	if row.status != "pending" || row.attempts != 1 || row.sealed == nil || row.lastErr == nil || *row.lastErr != "421 try again later" {
		t.Fatalf("depois da primeira falha: %+v", row)
	}

	outcome, err = repo.DeliverOutbox(ctx, id, true, fail)
	if err != nil || outcome != OutboxFailed {
		t.Fatalf("última falha: %v %v", outcome, err)
	}
	if row := readOutbox(t, repo, id); row.status != "failed" || row.attempts != 2 || row.sealed != nil {
		t.Fatalf("depois da última falha: %+v", row)
	}
}

// Só o que foi gravado vai para o OutboxTracker: o pedido recusado pelo intervalo de
// reenvio não anota nada, e a transação dele não deixa linha.
func TestOutboxTracksOnlyCommittedRows(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	email := outboxTestEmail(t, repo)
	newOTPOutbox(t, repo, email, "666666")

	ctx, tracker := WithOutboxTracker(context.Background())
	if err := repo.SaveOTP(ctx, email, "777777", OTPPurposeVerifyEmail, "en", OTPValidity); !errors.Is(err, ErrOTPCooldown) {
		t.Fatalf("segundo código dentro do intervalo: %v", err)
	}
	if ids := tracker.Drain(); len(ids) != 0 {
		t.Fatalf("pedido recusado anotou %v", ids)
	}
	var n int
	conn.QueryRow(context.Background(), `SELECT count(*) FROM email_outbox WHERE email = $1`, email).Scan(&n)
	if n != 1 {
		t.Fatalf("%d linhas na caixa de saída, want 1", n)
	}
}

// A varredura pega a pendente sem tarefa depois de um minuto, larga a que virou tarefa,
// e desiste do código vencido.
func TestUnqueuedOutbox(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()
	email := outboxTestEmail(t, repo)
	id := newOTPOutbox(t, repo, email, "888888")

	if ids, err := repo.UnqueuedOutbox(ctx); err != nil || slices.Contains(ids, id) {
		t.Fatalf("linha recém-criada já na varredura: %v", err)
	}
	conn.Exec(ctx, `UPDATE email_outbox SET created_at = created_at - interval '2 minutes' WHERE id = $1`, id)
	if ids, err := repo.UnqueuedOutbox(ctx); err != nil || !slices.Contains(ids, id) {
		t.Fatalf("pendente sem tarefa fora da varredura: %v", err)
	}
	if err := repo.MarkOutboxEnqueued(ctx, id); err != nil {
		t.Fatal(err)
	}
	if ids, err := repo.UnqueuedOutbox(ctx); err != nil || slices.Contains(ids, id) {
		t.Fatalf("linha com tarefa na varredura: %v", err)
	}

	conn.Exec(ctx, `UPDATE email_outbox SET created_at = created_at - interval '20 minutes' WHERE id = $1`, id)
	if _, err := repo.UnqueuedOutbox(ctx); err != nil {
		t.Fatal(err)
	}
	if row := readOutbox(t, repo, id); row.status != "skipped" || row.sealed != nil {
		t.Fatalf("código vencido ficou guardado: %+v", row)
	}
}

// As boas-vindas saem para o endereço da conta, e não saem para conta que pediu
// exclusão.
func TestOutboxWelcome(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()

	userID, email := newDeletionTestUser(t, repo)
	var welcome string
	if err := conn.QueryRow(ctx, `SELECT id FROM email_outbox WHERE user_id = $1 AND kind = 'welcome'`, userID).Scan(&welcome); err != nil {
		t.Fatalf("o cadastro não pôs as boas-vindas na caixa de saída: %v", err)
	}
	// Outra cópia, para o caso da conta em exclusão.
	var second string
	if err := conn.QueryRow(ctx, `
		INSERT INTO email_outbox (kind, locale, user_id) VALUES ('welcome', 'es', $1) RETURNING id`,
		userID).Scan(&second); err != nil {
		t.Fatal(err)
	}

	var to string
	if _, err := repo.DeliverOutbox(ctx, welcome, false, func(m OutboxMessage) error {
		to = m.Email
		return nil
	}); err != nil || to != email {
		t.Fatalf("boas-vindas para %q (%v), want %q", to, err, email)
	}

	if _, err := repo.MarkAccountForDeletion(ctx, userID); err != nil {
		t.Fatal(err)
	}
	outcome, err := repo.DeliverOutbox(ctx, second, false, func(OutboxMessage) error {
		t.Fatal("boas-vindas para conta em exclusão")
		return nil
	})
	if err != nil || outcome != OutboxSkipped {
		t.Fatalf("%v %v", outcome, err)
	}
}

// A poda leva o e-mail de código depois de um dia, que guarda o endereço em claro, e o
// resto depois de uma semana.
func TestPruneOutbox(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()
	email := outboxTestEmail(t, repo)
	old := newOTPOutbox(t, repo, email, "999999")
	conn.Exec(ctx, `UPDATE otps SET sent_at = sent_at - interval '2 minutes' WHERE email = $1`, email)
	fresh := newOTPOutbox(t, repo, email, "000001")
	age := func(id string, d time.Duration) {
		conn.Exec(ctx, `UPDATE email_outbox SET created_at = created_at - make_interval(secs => $2) WHERE id = $1`,
			id, d.Seconds())
	}
	age(old, OutboxOTPRetention+time.Hour)

	userID, _ := newDeletionTestUser(t, repo)
	var welcomeDay, welcomeWeek string
	conn.QueryRow(ctx, `SELECT id FROM email_outbox WHERE user_id = $1`, userID).Scan(&welcomeDay)
	conn.QueryRow(ctx, `INSERT INTO email_outbox (kind, locale, user_id) VALUES ('welcome', 'en', $1) RETURNING id`,
		userID).Scan(&welcomeWeek)
	age(welcomeDay, OutboxOTPRetention+time.Hour)
	age(welcomeWeek, OutboxRetention+time.Hour)

	if _, err := repo.PruneOutbox(ctx); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]int{old: 0, fresh: 1, welcomeDay: 1, welcomeWeek: 0} {
		var n int
		conn.QueryRow(ctx, `SELECT count(*) FROM email_outbox WHERE id = $1`, id).Scan(&n)
		if n != want {
			t.Errorf("linha %s: %d, want %d", id, n, want)
		}
	}
}

// A fila que desiste sem a rota gravar nada (erro de banco, 429, timeout) deixava a
// linha pendente. Uma hora depois de virar tarefa, a varredura a dá por perdida, e a
// inscrição da lista de espera volta a aceitar envio.
func TestExpireStuckOutbox(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()
	addr := newWaitlistEmail(t, repo)

	waitlistID, send, err := repo.JoinWaitlist(ctx, addr, "en")
	if err != nil || !send {
		t.Fatalf("inscrição: %v %v", send, err)
	}
	var stuck string
	conn.QueryRow(ctx, `SELECT id FROM email_outbox WHERE waitlist_id = $1`, waitlistID).Scan(&stuck)
	repo.MarkOutboxEnqueued(ctx, stuck)

	// Tarefa recente: a fila ainda está tentando.
	if _, err := repo.ExpireStuckOutbox(ctx); err != nil {
		t.Fatal(err)
	}
	if row := readOutbox(t, repo, stuck); row.status != "pending" {
		t.Fatalf("tarefa em andamento dada por perdida: %+v", row)
	}

	conn.Exec(ctx, `UPDATE email_outbox SET enqueued_at = enqueued_at - make_interval(secs => $2) WHERE id = $1`,
		stuck, (OutboxStuckAge + time.Minute).Seconds())
	if _, err := repo.ExpireStuckOutbox(ctx); err != nil {
		t.Fatal(err)
	}
	if row := readOutbox(t, repo, stuck); row.status != "failed" || row.lastErr == nil {
		t.Fatalf("linha presa: %+v", row)
	}
	if again, send, err := repo.JoinWaitlist(ctx, addr, "en"); err != nil || !send || again != waitlistID {
		t.Fatalf("a inscrição continuou travada: id=%q send=%v err=%v", again, send, err)
	}
}

// TRACK_KEY_SECRET trocada com código pendente: o código não sai, e a linha diz por quê.
func TestOutboxCodeUnderARotatedKey(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	email := outboxTestEmail(t, repo)
	id := newOTPOutbox(t, repo, email, "121212")

	saved := TrackKeySecret
	TrackKeySecret = []byte("another-track-key-secret-32bytes")
	t.Cleanup(func() { TrackKeySecret = saved })

	outcome, err := repo.DeliverOutbox(context.Background(), id, false, func(OutboxMessage) error {
		t.Fatal("código enviado com a chave errada")
		return nil
	})
	if err != nil || outcome != OutboxSkipped {
		t.Fatalf("%v %v", outcome, err)
	}
	if row := readOutbox(t, repo, id); row.status != "skipped" || row.sealed != nil || row.lastErr == nil {
		t.Fatalf("linha: %+v", row)
	}
}
