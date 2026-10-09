package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/josecleiton/logn/backend/internal/domain"
)

// Sem token não há placar, e a rota está ligada ao balde dela: o 61º pedido no minuto
// ouve 429 antes de chegar à autenticação.
func TestLeaderboardRouteNeedsTokenAndIsLimited(t *testing.T) {
	h := newTestHandler(t)
	for i := 0; i < 60; i++ {
		rr := serve(h, http.MethodGet, "/api/v1/leaderboard", "")
		if rr.Code != http.StatusUnauthorized || !strings.Contains(rr.Body.String(), codeUnauthenticated) {
			t.Fatalf("pedido %d sem token: got %d %s, want 401 %s", i+1, rr.Code, rr.Body.String(), codeUnauthenticated)
		}
	}
	rr := serve(h, http.MethodGet, "/api/v1/leaderboard", "")
	if rr.Code != http.StatusTooManyRequests || !strings.Contains(rr.Body.String(), codeRateLimited) {
		t.Fatalf("61º pedido: got %d %s, want 429 %s", rr.Code, rr.Body.String(), codeRateLimited)
	}
}

// placarUser cria uma conta de teste sem XP e devolve o id e o número.
func placarUser(t *testing.T, conn *pgxpool.Pool) (string, int) {
	t.Helper()
	ctx := context.Background()
	uid, anon := newTestUUID(t), testAnonNumber(t)
	if _, err := conn.Exec(ctx, `INSERT INTO users (id, email, anon_number) VALUES ($1, $2, $3)`,
		uid, "placar-"+uid[:8]+"@example.com", anon); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Exec(ctx, `DELETE FROM users WHERE id = $1`, uid) })
	return uid, anon
}

func putNickname(t *testing.T, s *Server, userID, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/profile/nickname", strings.NewReader(body))
	if userID != "" {
		req.Header.Set("Authorization", bearer(t, userID))
	}
	rr := httptest.NewRecorder()
	limitBody(nicknameBodyLimit, s.nicknameHandler)(rr, req)
	return rr
}

func TestNicknameRouteAnswersWithCodes(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	s := &Server{repo: domain.NewRepository(conn)}
	uid, anon := placarUser(t, conn)

	expect(t, putNickname(t, s, "", `{"nickname":"ana_dev"}`), http.StatusUnauthorized, codeUnauthenticated)
	expect(t, putNickname(t, s, uid, `{"nickname":"`+strings.Repeat("a", nicknameBodyLimit)+`"}`),
		http.StatusRequestEntityTooLarge, codeBodyTooLarge)
	expect(t, putNickname(t, s, uid, `{"nickname":"ana_dev","user_id":"`+newTestUUID(t)+`"}`),
		http.StatusBadRequest, codeInvalidRequest)
	expect(t, putNickname(t, s, uid, `{"nickname":"ab"}`), http.StatusBadRequest, codeNicknameInvalid)
	expect(t, putNickname(t, s, uid, `{"nickname":"jogador_4821"}`), http.StatusConflict, codeNicknameReserved)

	nick := "h_" + newTestUUID(t)[:8]
	rr := putNickname(t, s, uid, `{"nickname":" `+strings.ToUpper(nick)+`"}`)
	expect(t, rr, http.StatusOK, "")
	var got nicknameResponse
	json.Unmarshal(rr.Body.Bytes(), &got)
	if got.Nickname != nick {
		t.Fatalf("apelido gravado %q, esperava %q", got.Nickname, nick)
	}
	expect(t, putNickname(t, s, uid, `{"nickname":"outro_nome"}`), http.StatusConflict, codeNicknameLocked)

	other, _ := placarUser(t, conn)
	expect(t, putNickname(t, s, other, `{"nickname":"`+nick+`"}`), http.StatusConflict, codeNicknameTaken)

	// O Perfil lê o nome pelo /progress.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/progress", nil)
	req.Header.Set("Authorization", bearer(t, uid))
	prog := httptest.NewRecorder()
	s.getUserProgressHandler(prog, req)
	var stats domain.UserStats
	json.Unmarshal(prog.Body.Bytes(), &stats)
	if stats.AnonNumber != anon || stats.Nickname == nil || *stats.Nickname != nick || stats.NicknameLocked {
		t.Fatalf("/progress: anon=%d nickname=%v locked=%v", stats.AnonNumber, stats.Nickname, stats.NicknameLocked)
	}
	// O XP da conta não vai para o cache HTTP do aparelho: toda rota autenticada nasce
	// `no-store`.
	if cc := prog.Header().Get("Cache-Control"); cc != "private, no-store" {
		t.Fatalf("/progress saiu com Cache-Control %q", cc)
	}
}

// Além do balde por IP, cada conta tem dez tentativas por minuto.
func TestNicknameRouteLimitsEachAccount(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	s := &Server{repo: domain.NewRepository(conn), nicknameUserLimiter: newRateLimiter(10, time.Minute)}
	uid, _ := placarUser(t, conn)

	for i := 0; i < 10; i++ {
		expect(t, putNickname(t, s, uid, `{"nickname":"ab"}`), http.StatusBadRequest, codeNicknameInvalid)
	}
	expect(t, putNickname(t, s, uid, `{"nickname":"ab"}`), http.StatusTooManyRequests, codeRateLimited)
}

func leaderboardActionCall(t *testing.T, s *Server, caller string, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/internal/leaderboard/actions", strings.NewReader(body))
	if caller != "" {
		req.Header.Set("Authorization", "Bearer "+caller)
	}
	rr := httptest.NewRecorder()
	limitBody(licenseActionBodyLimit, s.leaderboardActionHandler)(rr, req)
	return rr
}

func TestOnlyTheAdminAccountModeratesTheLeaderboard(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	s := &Server{repo: domain.NewRepository(conn), cloudValidator: &mockCloudValidator{}}
	internalEnv(t, testAudience, testSchedulerAccount, testAdminAccount)
	uid, anon := placarUser(t, conn)
	body := `{"user_id":"` + uid + `","action":"hide","reason":"objeção por e-mail"}`

	for name, caller := range map[string]string{
		"sem token":          "",
		"outra conta":        "attacker@other-project.iam.gserviceaccount.com",
		"conta do Scheduler": testSchedulerAccount,
		"e-mail parecido":    testAdminAccount + ".evil",
	} {
		if rr := leaderboardActionCall(t, s, caller, body); rr.Code != http.StatusUnauthorized && rr.Code != http.StatusForbidden {
			t.Errorf("%s: got %d", name, rr.Code)
		}
	}
	t.Setenv(envAdminAccount, "")
	if rr := leaderboardActionCall(t, s, testAdminAccount, body); rr.Code != http.StatusForbidden {
		t.Fatalf("sem a conta de administração configurada: got %d", rr.Code)
	}
	internalEnv(t, "", testSchedulerAccount, testAdminAccount)
	if rr := leaderboardActionCall(t, s, testAdminAccount, body); rr.Code != http.StatusForbidden {
		t.Fatalf("sem a audiência configurada: got %d", rr.Code)
	}
	internalEnv(t, testAudience, testSchedulerAccount, testAdminAccount)

	var hidden bool
	conn.QueryRow(context.Background(), `SELECT leaderboard_hidden FROM users WHERE id = $1`, uid).Scan(&hidden)
	if hidden {
		t.Fatal("uma chamada recusada ocultou a conta")
	}

	for name, c := range map[string]struct {
		body   string
		status int
		code   string
	}{
		"actor no corpo":   {`{"user_id":"` + uid + `","action":"hide","reason":"r","actor":"eu"}`, http.StatusBadRequest, codeInvalidRequest},
		"user_id inválido": {`{"user_id":"x","action":"hide","reason":"r"}`, http.StatusBadRequest, codeInvalidRequest},
		"dois alvos":       {`{"user_id":"` + uid + `","anon_number":1,"action":"hide","reason":"r"}`, http.StatusBadRequest, codeInvalidRequest},
		"motivo vazio":     {`{"user_id":"` + uid + `","action":"hide","reason":"  "}`, http.StatusBadRequest, codeInvalidRequest},
		"motivo longo":     {`{"user_id":"` + uid + `","action":"hide","reason":"` + strings.Repeat("x", 2001) + `"}`, http.StatusBadRequest, codeInvalidRequest},
		"ação fora":        {`{"user_id":"` + uid + `","action":"delete","reason":"r"}`, http.StatusBadRequest, codeInvalidRequest},
		"conta que não há": {`{"user_id":"` + newTestUUID(t) + `","action":"hide","reason":"r"}`, http.StatusNotFound, codeUserNotFound},
		"corpo enorme":     {`{"user_id":"` + uid + `","action":"hide","reason":"` + strings.Repeat("x", licenseActionBodyLimit) + `"}`, http.StatusRequestEntityTooLarge, codeBodyTooLarge},
	} {
		rr := leaderboardActionCall(t, s, testAdminAccount, c.body)
		if rr.Code != c.status || errorCode(t, rr) != c.code {
			t.Errorf("%s: got %d %s, want %d %s", name, rr.Code, rr.Body.String(), c.status, c.code)
		}
	}

	expect(t, leaderboardActionCall(t, s, testAdminAccount, `{"anon_number":`+strconv.Itoa(anon)+`,"action":"hide","reason":"objeção por e-mail"}`), http.StatusOK, "")
	expect(t, leaderboardActionCall(t, s, testAdminAccount, body), http.StatusConflict, codeLeaderboardNoChange)
	var actor string
	conn.QueryRow(context.Background(), `SELECT actor FROM leaderboard_actions WHERE user_id = $1`, uid).Scan(&actor)
	if actor != testAdminAccount {
		t.Fatalf("autor gravado %q, esperava a conta do token", actor)
	}
}

// O token de uma conta que pediu exclusão não vê o placar.
func TestLeaderboardRefusesAccountPendingDeletion(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	ctx := context.Background()
	s := &Server{repo: domain.NewRepository(conn)}

	uid := newTestUUID(t)
	if _, err := conn.Exec(ctx, `
		INSERT INTO users (id, email, anon_number, deletion_requested_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP)`, uid, "placar-"+uid[:8]+"@example.com", testAnonNumber(t)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Exec(ctx, `DELETE FROM users WHERE id = $1`, uid) })

	req := httptest.NewRequest(http.MethodGet, "/api/v1/leaderboard", nil)
	req.Header.Set("Authorization", bearer(t, uid))
	rr := httptest.NewRecorder()
	s.leaderboardHandler(rr, req)
	if rr.Code != http.StatusUnauthorized || !strings.Contains(rr.Body.String(), codeUnauthenticated) {
		t.Fatalf("got %d %s, want 401 %s", rr.Code, rr.Body.String(), codeUnauthenticated)
	}
}

// A resposta não leva id nem e-mail de ninguém, nem do próprio jogador, e não fica em
// cache compartilhado.
func TestLeaderboardBodyHasNoUserIDOrEmail(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	ctx := context.Background()
	s := &Server{repo: domain.NewRepository(conn)}

	uid := newTestUUID(t)
	email := "placar-" + uid[:8] + "@example.com"
	if _, err := conn.Exec(ctx, `
		INSERT INTO users (id, email, anon_number, global_xp, free_xp, free_xp_reached_at)
		VALUES ($1, $2, $3, 500, 500, CURRENT_TIMESTAMP)`, uid, email, testAnonNumber(t)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Exec(ctx, `DELETE FROM users WHERE id = $1`, uid) })

	req := httptest.NewRequest(http.MethodGet, "/api/v1/leaderboard", nil)
	req.Header.Set("Authorization", bearer(t, uid))
	rr := httptest.NewRecorder()
	s.leaderboardHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("got %d %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if strings.Contains(body, uid) || strings.Contains(body, "@") {
		t.Fatalf("a resposta vazou id ou e-mail: %s", body)
	}
	var decoded map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"user_id", "id", "email"} {
		if _, ok := decoded[key]; ok {
			t.Fatalf("a resposta tem a chave %q", key)
		}
	}
	if got := rr.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
}
