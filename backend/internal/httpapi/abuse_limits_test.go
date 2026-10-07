package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/josecleiton/logn/backend/internal/domain"
)

// Um /64 de IPv6 é o que um assinante recebe. Contar o endereço inteiro dava a ele um
// balde novo a cada um dos 2^64 endereços.
func TestRateKeyGroupsIPv6By64(t *testing.T) {
	same := []string{"2001:db8:1:2::1", "2001:db8:1:2:ffff:ffff:ffff:ffff", "2001:db8:1:2:abcd::9"}
	for _, ip := range same[1:] {
		if rateKey(ip) != rateKey(same[0]) {
			t.Errorf("%s e %s são o mesmo /64 e caíram em baldes diferentes", ip, same[0])
		}
	}
	if rateKey("2001:db8:1:3::1") == rateKey("2001:db8:1:2::1") {
		t.Error("/64 diferentes caíram no mesmo balde")
	}
	if rateKey("203.0.113.7") == rateKey("203.0.113.8") {
		t.Error("IPv4 diferentes caíram no mesmo balde")
	}
	if rateKey("::ffff:203.0.113.7") != rateKey("203.0.113.7") {
		t.Error("IPv4 mapeado em IPv6 é o mesmo endereço")
	}

	rl := newRateLimiter(1, time.Minute)
	h := rl.wrap(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	for i, addr := range []string{"[2001:db8:1:2::1]:1000", "[2001:db8:1:2::2]:1000"} {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.RemoteAddr = addr
		rec := httptest.NewRecorder()
		h(rec, req)
		if i == 1 && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("segundo endereço do mesmo /64 passou: status %d", rec.Code)
		}
	}
}

// loginFixture cria uma conta com senha, só do teste, e apaga ela e as contagens dela
// no fim.
func loginFixture(t *testing.T, prefix string) (*Server, string, string) {
	t.Helper()
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	ctx := context.Background()
	repo := domain.NewRepository(conn)
	s := &Server{repo: repo}

	uid := newTestUUID(t)
	email := prefix + "-" + uid + "@example.com"
	hash, err := domain.HashPassword("senha-certa-123")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO users (id, email, password_hash, anon_number) VALUES ($1, $2, $3, $4)`, uid, email, hash, testAnonNumber(t)); err != nil {
		t.Fatalf("criando o usuário: %v", err)
	}
	t.Cleanup(func() {
		repo.ClearLoginAttempts(ctx, email)
		conn.Exec(ctx, `DELETE FROM otps WHERE email = $1`, email)
		conn.Exec(ctx, `DELETE FROM email_outbox WHERE kind = 'otp' AND email = $1`, email)
		conn.Exec(ctx, `DELETE FROM refresh_tokens WHERE user_id = $1`, uid)
		conn.Exec(ctx, `DELETE FROM users WHERE id = $1`, uid)
	})
	return s, email, uid
}

func doLogin(s *Server, email, password, remote string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	if remote != "" {
		req.RemoteAddr = remote
	}
	rec := httptest.NewRecorder()
	s.loginHandler(rec, req)
	return rec
}

// Passadas domain.LoginMaxAttemptsPerSource tentativas do mesmo IP, o login recusa com
// `login_locked` antes do Argon2, igual para conta que existe e para a que não existe:
// o 429 não diz quem tem conta. De outro IP, a dona da conta entra.
func TestLoginLockoutIsTheSameForAnyEmail(t *testing.T) {
	s, existing, _ := loginFixture(t, "lockout")
	missing := "missing-" + existing
	t.Cleanup(func() { s.repo.ClearLoginAttempts(context.Background(), missing) })

	for _, email := range []string{existing, missing} {
		for i := 1; i <= domain.LoginMaxAttemptsPerSource; i++ {
			if rec := doLogin(s, email, "senha-errada", ""); rec.Code != http.StatusUnauthorized {
				t.Fatalf("%s, tentativa %d: status %d", email, i, rec.Code)
			}
		}
		rec := doLogin(s, email, "senha-errada", "")
		if rec.Code != http.StatusTooManyRequests || decodeAPIError(t, rec).Code != codeLoginLocked {
			t.Fatalf("%s depois do teto: status %d (%s)", email, rec.Code, rec.Body.String())
		}
		if rec.Header().Get("Retry-After") == "" {
			t.Errorf("%s: 429 sem Retry-After", email)
		}
	}

	// Travada daqui, nem a senha certa entra daqui.
	if rec := doLogin(s, existing, "senha-certa-123", ""); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("senha certa do IP travado: status %d", rec.Code)
	}
	// De outro IP, entra: o chute alheio não tranca a dona da conta para fora.
	if rec := doLogin(s, existing, "senha-certa-123", "198.51.100.20:5000"); rec.Code != http.StatusOK {
		t.Fatalf("senha certa de outro IP: status %d (%s)", rec.Code, rec.Body.String())
	}
}

// O login certo zera as tentativas erradas de antes.
func TestLoginSuccessClearsTheAttempts(t *testing.T) {
	s, email, _ := loginFixture(t, "lockout-clear")

	for i := 1; i < domain.LoginMaxAttemptsPerSource; i++ {
		doLogin(s, email, "senha-errada", "")
	}
	if rec := doLogin(s, email, "senha-certa-123", ""); rec.Code != http.StatusOK {
		t.Fatalf("senha certa dentro do teto: status %d", rec.Code)
	}
	for i := 1; i <= domain.LoginMaxAttemptsPerSource; i++ {
		if rec := doLogin(s, email, "senha-errada", ""); rec.Code != http.StatusUnauthorized {
			t.Fatalf("depois de zerar, tentativa %d: status %d", i, rec.Code)
		}
	}
}

// Trocar a senha pelo código destrava o login: quem provou que é dono do e-mail entra
// com a senha nova na hora, mesmo travado pelo chute de outra pessoa.
func TestResetPasswordClearsTheLockout(t *testing.T) {
	s, email, _ := loginFixture(t, "lockout-reset")
	ctx := context.Background()

	for i := 0; i <= domain.LoginMaxAttemptsPerSource; i++ {
		doLogin(s, email, "senha-errada", "")
	}
	if rec := doLogin(s, email, "senha-certa-123", ""); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("a conta devia estar travada: status %d", rec.Code)
	}

	if err := s.repo.SaveOTP(ctx, email, "123456", domain.OTPPurposeResetPassword, "pt-BR", domain.OTPValidity); err != nil {
		t.Fatalf("código de troca: %v", err)
	}
	body, _ := json.Marshal(map[string]string{"email": email, "otp": "123456", "password": "senha-nova-456"})
	rec := httptest.NewRecorder()
	s.resetPasswordHandler(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("troca de senha: status %d (%s)", rec.Code, rec.Body.String())
	}
	if rec := doLogin(s, email, "senha-nova-456", ""); rec.Code != http.StatusOK {
		t.Fatalf("senha nova depois da troca: status %d (%s)", rec.Code, rec.Body.String())
	}
}

// O envio de código tem balde por IP, na frente do teto global, e a janela de falhas
// estourada responde `otp_locked` com o tempo que falta. Os dois saem antes de gravar
// código ou mandar e-mail.
func TestRequestOTPLimits(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	ctx := context.Background()
	s := &Server{repo: domain.NewRepository(conn)}

	email := "otp-limits-" + newTestUUID(t) + "@example.com"
	t.Cleanup(func() {
		conn.Exec(ctx, `DELETE FROM otps WHERE email = $1`, email)
		conn.Exec(ctx, `DELETE FROM email_outbox WHERE kind = 'otp' AND email = $1`, email)
	})
	request := func() *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"email": email, "purpose": domain.OTPPurposeResetPassword})
		rec := httptest.NewRecorder()
		s.requestOTPHandler(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/request-otp", bytes.NewReader(body)))
		return rec
	}

	// Janela de falhas estourada.
	if _, err := conn.Exec(ctx, `
		INSERT INTO otps (email, purpose, code_hash, expires_at, attempts, sent_at, recent_failures, failures_since)
		VALUES ($1, $2, 'x', CURRENT_TIMESTAMP, 0, CURRENT_TIMESTAMP - interval '1 hour', $3, CURRENT_TIMESTAMP)`,
		email, domain.OTPPurposeResetPassword, domain.OTPMaxFailuresPerWindow); err != nil {
		t.Fatalf("linha travada: %v", err)
	}
	rec := request()
	if rec.Code != http.StatusTooManyRequests || decodeAPIError(t, rec).Code != codeOTPLocked {
		t.Fatalf("janela estourada: status %d (%s)", rec.Code, rec.Body.String())
	}
	if secs, _ := strconv.Atoi(rec.Header().Get("Retry-After")); secs < int((domain.OTPFailureWindow - time.Hour).Seconds()) {
		t.Fatalf("Retry-After devia ser o que falta da janela, veio %q", rec.Header().Get("Retry-After"))
	}

	// Balde do IP gasto: o código é o do reenvio, que trava só o envio no app.
	s.otpSendLimiter = newRateLimiter(1, time.Hour)
	s.otpSendLimiter.allow(rateKey("192.0.2.1"))
	rec = request()
	if rec.Code != http.StatusTooManyRequests || decodeAPIError(t, rec).Code != codeOTPResendTooSoon {
		t.Fatalf("balde do IP gasto: status %d (%s)", rec.Code, rec.Body.String())
	}
}

// Sync acima do teto de eventos, com evento que o Core não escreve, além do ritmo da
// conta ou sem vaga é recusado antes de tocar na cadeia.
func TestSyncRefusesWhatPassesTheLimits(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	ctx := context.Background()
	s := &Server{repo: domain.NewRepository(conn)}

	uid := newTestUUID(t)
	if _, err := conn.Exec(ctx, `INSERT INTO users (id, email, anon_number) VALUES ($1, $2, $3)`, uid, "sync-limits-"+uid+"@example.com", testAnonNumber(t)); err != nil {
		t.Fatalf("criando o usuário: %v", err)
	}
	t.Cleanup(func() {
		conn.Exec(ctx, `DELETE FROM game_events WHERE user_id = $1`, uid)
		conn.Exec(ctx, `DELETE FROM user_sync_state WHERE user_id = $1`, uid)
		conn.Exec(ctx, `DELETE FROM users WHERE id = $1`, uid)
	})
	auth := bearer(t, uid)

	sync := func(events []domain.GameEvent) *httptest.ResponseRecorder {
		body, _ := json.Marshal(domain.SyncPayload{Events: events})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sync", bytes.NewReader(body))
		req.Header.Set("Authorization", auth)
		rec := httptest.NewRecorder()
		s.syncHandler(rec, req)
		return rec
	}
	chain := func(n int, eventType string) []domain.GameEvent {
		events := make([]domain.GameEvent, n)
		previous := strings.Repeat("0", 64)
		for i := range events {
			e := domain.GameEvent{ID: "limit_" + uid + "_" + strconv.Itoa(i),
				EventType: eventType, PayloadJSON: "{}", Timestamp: int64(1600000000 + i), PreviousHash: previous}
			e.CurrentHash = domain.ComputeHash(e, previous)
			previous = e.CurrentHash
			events[i] = e
		}
		return events
	}

	rec := sync(chain(domain.MaxSyncEvents+1, "MATCH_ANSWER"))
	if rec.Code != http.StatusRequestEntityTooLarge || decodeAPIError(t, rec).Code != codeSyncTooLarge {
		t.Fatalf("acima do teto: status %d (%s)", rec.Code, rec.Body.String())
	}
	rec = sync(chain(1, "GRANT_XP"))
	if rec.Code != http.StatusForbidden || decodeAPIError(t, rec).Code != codeSyncRejected {
		t.Fatalf("tipo desconhecido: status %d (%s)", rec.Code, rec.Body.String())
	}
	var stored int
	conn.QueryRow(ctx, `SELECT count(*) FROM game_events WHERE user_id = $1`, uid).Scan(&stored)
	if stored != 0 {
		t.Fatalf("sync recusado gravou %d eventos", stored)
	}

	// Todas as vagas ocupadas: a espera tem fim, e a vaga não vaza.
	previousWait := syncSlotWait
	syncSlotWait = 50 * time.Millisecond
	t.Cleanup(func() { syncSlotWait = previousWait })
	for i := 0; i < cap(syncSlots); i++ {
		syncSlots <- struct{}{}
	}
	rec = sync(nil)
	for i := 0; i < cap(syncSlots); i++ {
		<-syncSlots
	}
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("sem vaga: status %d", rec.Code)
	}
	if len(syncSlots) != 0 {
		t.Fatalf("o pedido sem vaga deixou %d vagas presas", len(syncSlots))
	}

	s.syncUserLimiter = newRateLimiter(1, time.Minute)
	if rec := sync(nil); rec.Code != http.StatusOK {
		t.Fatalf("primeiro sync da conta: status %d", rec.Code)
	}
	if len(syncSlots) != 0 {
		t.Fatal("o sync que terminou não devolveu a vaga")
	}
	rec = sync(nil)
	if rec.Code != http.StatusTooManyRequests || decodeAPIError(t, rec).Code != codeRateLimited {
		t.Fatalf("além do ritmo da conta: status %d", rec.Code)
	}
}

// O prazo de leitura do corpo chega à conexão também atrás do gzip. Sem `Unwrap` no
// writer do gzip, ele virava ErrNotSupported em todo pedido do app, calado.
func TestReadDeadlineReachesTheConnectionThroughGzip(t *testing.T) {
	var deadlineErr error
	srv := httptest.NewServer(withGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deadlineErr = http.NewResponseController(w).SetReadDeadline(time.Now().Add(time.Second))
		w.WriteHeader(http.StatusOK)
	})))
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader("{}"))
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if deadlineErr != nil {
		t.Fatalf("prazo de leitura atrás do gzip: %v", deadlineErr)
	}
}

// O user_id do corpo que não bate com o token vai para o log cortado e escapado: cru,
// um corpo grande virava log grande, e uma quebra de linha forjava outra entrada.
func TestSyncLogsAForeignUserIDClippedAndEscaped(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	ctx := context.Background()
	s := &Server{repo: domain.NewRepository(conn)}

	uid := newTestUUID(t)
	if _, err := conn.Exec(ctx, `INSERT INTO users (id, email, anon_number) VALUES ($1, $2, $3)`, uid, "sync-log-"+uid+"@example.com", testAnonNumber(t)); err != nil {
		t.Fatalf("criando o usuário: %v", err)
	}
	t.Cleanup(func() {
		conn.Exec(ctx, `DELETE FROM user_sync_state WHERE user_id = $1`, uid)
		conn.Exec(ctx, `DELETE FROM users WHERE id = $1`, uid)
	})

	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	forged := "x\n2026/10/02 00:00:00 sync ok: user=admin\n" + strings.Repeat("a", 100_000)
	body, _ := json.Marshal(domain.SyncPayload{UserID: forged})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sync", bytes.NewReader(body))
	req.Header.Set("Authorization", bearer(t, uid))
	s.syncHandler(httptest.NewRecorder(), req)

	out := buf.String()
	if !strings.Contains(out, "cheat attempt") {
		t.Fatalf("o log da tentativa não saiu: %q", out)
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.Contains(line, "sync ok: user=admin") && !strings.Contains(line, "cheat attempt") {
			t.Fatalf("a quebra de linha do cliente forjou uma entrada: %q", line)
		}
	}
	if len(out) > 1000 {
		t.Fatalf("o log levou %d bytes do corpo", len(out))
	}
}

// O corpo do sync tem teto na rota, e ele baixou de 8 MB.
func TestSyncBodyLimitIsSmall(t *testing.T) {
	if syncBodyLimit > 2<<20 {
		t.Fatalf("teto do sync em %d bytes", syncBodyLimit)
	}
	called := false
	h := limitBody(syncBodyLimit, func(w http.ResponseWriter, r *http.Request) {
		called = true
		var v any
		if err := json.NewDecoder(r.Body).Decode(&v); err == nil {
			t.Error("corpo acima do teto foi lido inteiro")
		}
	})
	big := `{"events":"` + strings.Repeat("a", syncBodyLimit) + `"}`
	h(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/v1/sync", strings.NewReader(big)))
	if !called {
		t.Fatal("handler não rodou")
	}
}
