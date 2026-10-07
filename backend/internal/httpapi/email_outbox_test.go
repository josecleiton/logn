package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/josecleiton/logn/backend/internal/domain"
)

// O código da caixa de saída é cifrado com a chave derivada de TRACK_KEY_SECRET, e o
// HMAC do OTP usa a do JWT. As duas ficam fixas no pacote: um código gravado com uma e
// conferido com outra seria descartado como trocado.
func TestMain(m *testing.M) {
	domain.TrackKeySecret = []byte("test-track-key-secret-32-bytes!!")
	domain.JwtSecretKey = []byte("test-secret")
	os.Exit(m.Run())
}

const testTasksAccount = "logn-tasks@example-project.iam.gserviceaccount.com"

type sentOTP struct{ to, purpose, code, lang string }

// fakeOutboxMailer guarda o que seria enviado.
type fakeOutboxMailer struct {
	mu       sync.Mutex
	otps     []sentOTP
	welcomes []string
	fail     bool
	// Não nulo, o envio de código espera ele fechar ou o ctx acabar: é o SMTP que segura
	// a conexão, e o Mailer, que desiste com o ctx.
	hang chan struct{}
}

func (m *fakeOutboxMailer) SendOTP(ctx context.Context, to, purpose, code, lang string) error {
	if m.hang != nil {
		select {
		case <-m.hang:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return errors.New("550 <" + to + ">: mailbox unavailable")
	}
	m.otps = append(m.otps, sentOTP{to, purpose, code, lang})
	return nil
}

func (m *fakeOutboxMailer) SendWelcome(_ context.Context, to, lang string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return errors.New("smtp fora")
	}
	m.welcomes = append(m.welcomes, to)
	return nil
}

// fakeQueue guarda as tarefas criadas, ou falha.
type fakeQueue struct {
	mu    sync.Mutex
	ids   []string
	err   error
	calls int
}

func (q *fakeQueue) Enqueue(_ context.Context, id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.calls++
	if q.err != nil {
		return q.err
	}
	q.ids = append(q.ids, id)
	return nil
}

type outboxFixture struct {
	s      *Server
	pool   *pgxpool.Pool
	mailer *fakeOutboxMailer
	email  string
}

func newOutboxFixture(t *testing.T, queue OutboxQueue) *outboxFixture {
	t.Helper()
	pool := setupTestDB(t)
	t.Cleanup(pool.Close)
	internalEnv(t, testAudience, testSchedulerAccount, testAdminAccount)
	t.Setenv(envTasksAccount, testTasksAccount)
	mailer := &fakeOutboxMailer{}
	f := &outboxFixture{
		s: &Server{
			repo:           domain.NewRepository(pool),
			cloudValidator: &mockCloudValidator{},
			outboxMailer:   mailer,
			outboxQueue:    queue,
		},
		pool:   pool,
		mailer: mailer,
		email:  "outbox-" + newTestUUID(t)[:12] + "@example.com",
	}
	t.Cleanup(func() {
		ctx := context.Background()
		pool.Exec(ctx, `DELETE FROM otps WHERE email = $1`, f.email)
		pool.Exec(ctx, `DELETE FROM email_outbox WHERE kind = 'otp' AND email = $1`, f.email)
	})
	return f
}

// requestOTP pede um código pela rota, com o middleware da caixa de saída em volta.
func (f *outboxFixture) requestOTP(t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": f.email, "purpose": domain.OTPPurposeVerifyEmail})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/request-otp", bytes.NewReader(body))
	req.Header.Set("Accept-Language", "es")
	rec := httptest.NewRecorder()
	f.s.withOutbox(http.HandlerFunc(f.s.requestOTPHandler)).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("request-otp: %d %s", rec.Code, rec.Body.String())
	}
	return rec
}

// outboxID é a linha de código mais nova do endereço.
func (f *outboxFixture) outboxID(t *testing.T) string {
	t.Helper()
	var id string
	if err := f.pool.QueryRow(context.Background(), `
		SELECT id FROM email_outbox WHERE kind = 'otp' AND email = $1
		ORDER BY created_at DESC LIMIT 1`, f.email).Scan(&id); err != nil {
		t.Fatalf("linha da caixa de saída: %v", err)
	}
	return id
}

func (f *outboxFixture) status(t *testing.T, id string) (status string, enqueued bool) {
	t.Helper()
	if err := f.pool.QueryRow(context.Background(), `
		SELECT status, enqueued_at IS NOT NULL FROM email_outbox WHERE id = $1`, id).Scan(&status, &enqueued); err != nil {
		t.Fatal(err)
	}
	return status, enqueued
}

// O pedido grava o código e, quando termina, põe o e-mail na fila. Quem envia é a
// tarefa, não o pedido.
func TestRequestOTPQueuesTheEmail(t *testing.T) {
	queue := &fakeQueue{}
	f := newOutboxFixture(t, queue)
	f.requestOTP(t)

	id := f.outboxID(t)
	if !slices.Equal(queue.ids, []string{id}) {
		t.Fatalf("fila recebeu %v, want [%s]", queue.ids, id)
	}
	if status, enqueued := f.status(t, id); status != "pending" || !enqueued {
		t.Fatalf("linha: %s enqueued=%v", status, enqueued)
	}
	if len(f.mailer.otps) != 0 {
		t.Fatal("o pedido mandou o e-mail em vez da fila")
	}
}

// Fila fora do ar não derruba o pedido: o código foi gravado, e a linha fica para a
// varredura.
func TestRequestOTPWithTheQueueDown(t *testing.T) {
	f := newOutboxFixture(t, &fakeQueue{err: errors.New("cloud tasks respondeu 503")})
	f.requestOTP(t)
	if status, enqueued := f.status(t, f.outboxID(t)); status != "pending" || enqueued {
		t.Fatalf("linha: %s enqueued=%v", status, enqueued)
	}
}

// Sem fila (desenvolvimento), o e-mail sai na hora, ainda dentro do pedido.
func TestRequestOTPWithoutAQueueSendsInline(t *testing.T) {
	f := newOutboxFixture(t, nil)
	f.requestOTP(t)
	if len(f.mailer.otps) != 1 || f.mailer.otps[0].to != f.email || f.mailer.otps[0].lang != "es" {
		t.Fatalf("e-mails: %+v", f.mailer.otps)
	}
	if status, _ := f.status(t, f.outboxID(t)); status != "sent" {
		t.Fatalf("linha: %s", status)
	}
}

func sendRequest(id, auth string, retries int) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/internal/email/send", strings.NewReader(`{"id":"`+id+`"}`))
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	req.Header.Set("X-CloudTasks-TaskRetryCount", strconv.Itoa(retries))
	return req
}

// A entrega só aceita o token da conta do Cloud Tasks. A da purga e a de administração
// têm token válido do Google com a mesma audiência, e são recusadas.
func TestEmailSendHandlerAuthorization(t *testing.T) {
	f := newOutboxFixture(t, &fakeQueue{})
	f.requestOTP(t)
	id := f.outboxID(t)

	for _, tc := range []struct {
		name    string
		account string
		auth    string
		fail    bool
		want    int
	}{
		{"sem conta configurada", "", "Bearer " + testTasksAccount, false, http.StatusForbidden},
		{"sem cabeçalho", testTasksAccount, "", false, http.StatusUnauthorized},
		{"token inválido", testTasksAccount, "Bearer " + testTasksAccount, true, http.StatusUnauthorized},
		{"conta da purga", testTasksAccount, "Bearer " + testSchedulerAccount, false, http.StatusForbidden},
		{"conta de administração", testTasksAccount, "Bearer " + testAdminAccount, false, http.StatusForbidden},
		{"conta de outro projeto", testTasksAccount, "Bearer attacker@other-project.iam.gserviceaccount.com", false, http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(envTasksAccount, tc.account)
			f.s.cloudValidator = &mockCloudValidator{shouldFail: tc.fail}
			rec := httptest.NewRecorder()
			f.s.emailSendHandler(rec, sendRequest(id, tc.auth, 0))
			if rec.Code != tc.want {
				t.Fatalf("status %d, want %d", rec.Code, tc.want)
			}
		})
	}
	if len(f.mailer.otps) != 0 {
		t.Fatal("pedido recusado mandou e-mail")
	}
	if status, _ := f.status(t, id); status != "pending" {
		t.Fatalf("pedido recusado mexeu na linha: %s", status)
	}
}

// Corpo que não é `{"id": <uuid>}` é 400, sem tocar no banco.
func TestEmailSendHandlerRejectsABadBody(t *testing.T) {
	f := newOutboxFixture(t, &fakeQueue{})
	handler := limitBody(outboxSendBodyLimit, f.s.emailSendHandler)
	for name, body := range map[string]string{
		"campo desconhecido": `{"id":"0b8e7c9a-1f2d-4e3b-9a8c-7d6e5f4a3b2c","to":"x@example.com"}`,
		"id que não é uuid":  `{"id":"1; DROP TABLE users"}`,
		"sem id":             `{}`,
		"acima do teto":      `{"id":"` + strings.Repeat("a", outboxSendBodyLimit) + `"}`,
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/internal/email/send", strings.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+testTasksAccount)
			rec := httptest.NewRecorder()
			handler(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status %d", rec.Code)
			}
		})
	}
}

// A tarefa entregue duas vezes manda um e-mail só, com o código e na língua do pedido.
func TestEmailSendHandlerDeliversOnce(t *testing.T) {
	f := newOutboxFixture(t, &fakeQueue{})
	f.requestOTP(t)
	id := f.outboxID(t)

	for range 2 {
		rec := httptest.NewRecorder()
		f.s.emailSendHandler(rec, sendRequest(id, "Bearer "+testTasksAccount, 0))
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
	}
	if len(f.mailer.otps) != 1 {
		t.Fatalf("%d e-mails, want 1", len(f.mailer.otps))
	}
	got := f.mailer.otps[0]
	if got.to != f.email || got.lang != "es" || len(got.code) != 6 {
		t.Fatalf("e-mail: %+v", got)
	}
}

// SMTP fora antes da última tentativa é 503, e a fila tenta de novo; na última, 200 e
// a linha desiste. O endereço que o SMTP repete na recusa não vai para a linha.
func TestEmailSendHandlerRetriesThenGivesUp(t *testing.T) {
	f := newOutboxFixture(t, &fakeQueue{})
	f.requestOTP(t)
	id := f.outboxID(t)
	f.mailer.fail = true

	rec := httptest.NewRecorder()
	f.s.emailSendHandler(rec, sendRequest(id, "Bearer "+testTasksAccount, 0))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("primeira falha: status %d", rec.Code)
	}
	var lastErr string
	f.pool.QueryRow(context.Background(), `SELECT last_error FROM email_outbox WHERE id = $1`, id).Scan(&lastErr)
	if strings.Contains(lastErr, "@") {
		t.Fatalf("last_error guardou o endereço: %q", lastErr)
	}

	rec = httptest.NewRecorder()
	f.s.emailSendHandler(rec, sendRequest(id, "Bearer "+testTasksAccount, OutboxMaxAttempts-1))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "failed") {
		t.Fatalf("última tentativa: %d %s", rec.Code, rec.Body.String())
	}
	if status, _ := f.status(t, id); status != "failed" {
		t.Fatalf("linha: %s", status)
	}
}

// SMTP que segura a conexão não prende a linha nem a conexão do pool: passado o prazo,
// a entrega conta como falha e a fila tenta de novo.
func TestEmailSendHandlerGivesUpOnAHangingSMTP(t *testing.T) {
	f := newOutboxFixture(t, &fakeQueue{})
	f.requestOTP(t)
	id := f.outboxID(t)
	saved := smtpSendTimeout
	smtpSendTimeout = 50 * time.Millisecond
	t.Cleanup(func() { smtpSendTimeout = saved })
	f.mailer.hang = make(chan struct{})
	t.Cleanup(func() { close(f.mailer.hang) })

	rec := httptest.NewRecorder()
	f.s.emailSendHandler(rec, sendRequest(id, "Bearer "+testTasksAccount, 0))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", rec.Code)
	}
	var status, lastErr string
	f.pool.QueryRow(context.Background(), `SELECT status, last_error FROM email_outbox WHERE id = $1`, id).Scan(&status, &lastErr)
	if status != "pending" || !strings.Contains(lastErr, "prazo") {
		t.Fatalf("linha: %s %q", status, lastErr)
	}
}

// A varredura só aceita a conta do Scheduler, e põe na fila a pendente que ficou sem
// tarefa.
func TestEmailSweepHandler(t *testing.T) {
	queue := &fakeQueue{err: errors.New("fora do ar")}
	f := newOutboxFixture(t, queue)
	f.requestOTP(t)
	id := f.outboxID(t)
	f.pool.Exec(context.Background(), `UPDATE email_outbox SET created_at = created_at - interval '2 minutes' WHERE id = $1`, id)
	queue.err, queue.calls = nil, 0

	sweep := func(auth string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/internal/email/sweep", nil)
		req.Header.Set("Authorization", auth)
		rec := httptest.NewRecorder()
		f.s.emailSweepHandler(rec, req)
		return rec.Code
	}
	if code := sweep("Bearer " + testTasksAccount); code != http.StatusForbidden {
		t.Fatalf("conta do Cloud Tasks na varredura: %d", code)
	}
	if queue.calls != 0 {
		t.Fatal("varredura recusada pôs na fila")
	}

	// Fila fora do ar: a varredura desiste na primeira recusa, em vez de esperar o prazo
	// de cada pendente.
	queue.err = errors.New("fora do ar")
	if code := sweep("Bearer " + testSchedulerAccount); code != http.StatusOK {
		t.Fatalf("varredura com a fila fora: %d", code)
	}
	if queue.calls != 1 {
		t.Fatalf("a varredura insistiu na fila fora do ar: %d chamadas", queue.calls)
	}
	queue.err = nil
	if code := sweep("Bearer " + testSchedulerAccount); code != http.StatusOK {
		t.Fatalf("varredura: %d", code)
	}
	if !slices.Contains(queue.ids, id) {
		t.Fatalf("a varredura não levou %s: %v", id, queue.ids)
	}
	if _, enqueued := f.status(t, id); !enqueued {
		t.Fatal("a varredura não registrou a tarefa")
	}
}
