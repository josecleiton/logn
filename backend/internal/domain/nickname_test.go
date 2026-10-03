package domain

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNormalizeNickname(t *testing.T) {
	for _, c := range []struct {
		raw, want string
		err       error
	}{
		{"ana_dev", "ana_dev", nil},
		{"  Ana_Dev  ", "ana_dev", nil},
		{"abc", "abc", nil},
		{strings.Repeat("a", 20), strings.Repeat("a", 20), nil},
		{"ab", "", ErrNicknameInvalid},
		{strings.Repeat("a", 21), "", ErrNicknameInvalid},
		{"joão", "", ErrNicknameInvalid},
		{"ana dev", "", ErrNicknameInvalid},
		{"ana-dev", "", ErrNicknameInvalid},
		// O "K" do Kelvin (U+212A) não vira "k".
		{"Kbfs", "", ErrNicknameInvalid},
		{"", "", ErrNicknameInvalid},
	} {
		got, err := NormalizeNickname(c.raw)
		if got != c.want || !errors.Is(err, c.err) {
			t.Errorf("NormalizeNickname(%q) = %q, %v; esperava %q, %v", c.raw, got, err, c.want, c.err)
		}
	}
}

func TestReservedNickname(t *testing.T) {
	for _, n := range []string{"gm", "gm_1", "logn2", "jogador_4821", "admin", "suporte__", "player007"} {
		if !reservedNickname(n) {
			t.Errorf("%q tinha de ser reservado", n)
		}
	}
	for _, n := range []string{"modesto", "teamaker", "gmail", "the_gm", "ana_dev", "rootbeer"} {
		if reservedNickname(n) {
			t.Errorf("%q não é reservado", n)
		}
	}
}

func TestSetNicknameIsAOneTimeChoice(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()
	u := seedBoardUser(t, conn, 0, time.Time{})

	nick := "n_" + testUUID(t)[:8]
	got, err := repo.SetNickname(ctx, u.id, "  "+strings.ToUpper(nick)+" ")
	if err != nil || got != nick {
		t.Fatalf("primeira escolha: %q, %v", got, err)
	}
	if _, err := repo.SetNickname(ctx, u.id, "n_"+testUUID(t)[:8]); !errors.Is(err, ErrNicknameLocked) {
		t.Fatalf("segunda escolha: %v, esperava ErrNicknameLocked", err)
	}
	// "gm" sozinho tem 2 letras e já cai no formato; com sufixo, cai na lista.
	other := seedBoardUser(t, conn, 0, time.Time{})
	if _, err := repo.SetNickname(ctx, other.id, " GM_1 "); !errors.Is(err, ErrNicknameReserved) {
		t.Fatalf("\" GM_1 \": %v, esperava ErrNicknameReserved", err)
	}
}

// Dois pedidos com o mesmo apelido ao mesmo tempo: o índice decide, e um deles ouve
// "em uso".
func TestSetNicknameRaceGivesItToOne(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()
	a := seedBoardUser(t, conn, 0, time.Time{})
	b := seedBoardUser(t, conn, 0, time.Time{})

	nick := "r_" + testUUID(t)[:8]
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, 2)
	for i, id := range []string{a.id, b.id} {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			<-start
			_, errs[i] = repo.SetNickname(ctx, id, nick)
		}(i, id)
	}
	close(start)
	wg.Wait()

	ok, taken := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, ErrNicknameTaken):
			taken++
		default:
			t.Fatalf("erro inesperado: %v", err)
		}
	}
	if ok != 1 || taken != 1 {
		t.Fatalf("na corrida: %d gravaram e %d ouviram em uso; esperava 1 e 1", ok, taken)
	}
}

func moderate(t *testing.T, repo *Repository, target LeaderboardTarget, action string) error {
	t.Helper()
	_, err := repo.ApplyLeaderboardAction(context.Background(), LeaderboardAction{
		Target: target, Action: action, Reason: "teste de moderação", Actor: "logn-admin@example-project.iam.gserviceaccount.com",
	})
	return err
}

// Anonimizar queima a chance da conta e bloqueia o apelido para todo mundo, inclusive
// depois que a dona exclui a conta.
func TestAnonymizeBurnsTheChanceAndBlocksTheNickname(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()
	owner := seedBoardUser(t, conn, 0, time.Time{})
	other := seedBoardUser(t, conn, 0, time.Time{})

	nick := "o_" + testUUID(t)[:8]
	t.Cleanup(func() { conn.Exec(ctx, `DELETE FROM blocked_nicknames WHERE nickname = $1`, nick) })
	if _, err := repo.SetNickname(ctx, owner.id, nick); err != nil {
		t.Fatal(err)
	}
	if err := moderate(t, repo, LeaderboardTarget{Nickname: nick}, LeaderboardAnonymize); err != nil {
		t.Fatal(err)
	}

	stats, err := repo.GetUserStats(ctx, owner.id)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Nickname != nil || !stats.NicknameLocked || stats.AnonNumber != owner.anon {
		t.Fatalf("depois de anonimizar: nickname=%v locked=%v anon=%d", stats.Nickname, stats.NicknameLocked, stats.AnonNumber)
	}
	if _, err := repo.SetNickname(ctx, owner.id, "n_"+testUUID(t)[:8]); !errors.Is(err, ErrNicknameLocked) {
		t.Fatalf("a dona escolheu de novo: %v", err)
	}
	if _, err := repo.SetNickname(ctx, other.id, nick); !errors.Is(err, ErrNicknameReserved) {
		t.Fatalf("outra conta pegou o apelido moderado: %v", err)
	}

	conn.Exec(ctx, `DELETE FROM users WHERE id = $1`, owner.id)
	if _, err := repo.SetNickname(ctx, other.id, nick); !errors.Is(err, ErrNicknameReserved) {
		t.Fatalf("o apelido moderado voltou depois da exclusão da dona: %v", err)
	}
	if err := moderate(t, repo, LeaderboardTarget{AnonNumber: other.anon}, LeaderboardAnonymize); err != nil {
		t.Fatalf("anonimizar quem nunca teve apelido queima a chance: %v", err)
	}
	// Travada, a conta ouve "travada" até para um apelido bloqueado: ela não sonda a
	// lista da moderação.
	if _, err := repo.SetNickname(ctx, other.id, nick); !errors.Is(err, ErrNicknameLocked) {
		t.Fatalf("conta travada sondando apelido bloqueado: %v, esperava ErrNicknameLocked", err)
	}
	if err := moderate(t, repo, LeaderboardTarget{AnonNumber: other.anon}, LeaderboardAnonymize); !errors.Is(err, ErrLeaderboardNoChange) {
		t.Fatalf("anonimizar de novo: %v, esperava ErrLeaderboardNoChange", err)
	}
}

func TestHideAndUnhideRecordWhoDidIt(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()
	u := seedBoardUser(t, conn, 0, time.Time{})
	email := "placar-" + u.id[:8] + "@example.com"

	if err := moderate(t, repo, LeaderboardTarget{Email: " " + strings.ToUpper(email)}, LeaderboardHide); err != nil {
		t.Fatal(err)
	}
	if err := moderate(t, repo, LeaderboardTarget{UserID: u.id}, LeaderboardHide); !errors.Is(err, ErrLeaderboardNoChange) {
		t.Fatalf("ocultar duas vezes: %v", err)
	}
	if err := moderate(t, repo, LeaderboardTarget{UserID: u.id}, LeaderboardUnhide); err != nil {
		t.Fatal(err)
	}
	var actions int
	var actor string
	conn.QueryRow(ctx, `SELECT count(*), max(actor) FROM leaderboard_actions WHERE user_id = $1`, u.id).Scan(&actions, &actor)
	if actions != 2 || actor != "logn-admin@example-project.iam.gserviceaccount.com" {
		t.Fatalf("histórico: %d ações, autor %q", actions, actor)
	}
}

func TestLeaderboardActionNeedsExactlyOneTargetAndAReason(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	u := seedBoardUser(t, conn, 0, time.Time{})

	for name, target := range map[string]LeaderboardTarget{
		"nenhum": {},
		"dois":   {UserID: u.id, AnonNumber: u.anon},
	} {
		if err := moderate(t, repo, target, LeaderboardHide); !errors.Is(err, ErrInvalidLeaderboardAction) {
			t.Errorf("alvo %s: %v", name, err)
		}
	}
	if err := moderate(t, repo, LeaderboardTarget{UserID: u.id}, "delete"); !errors.Is(err, ErrInvalidLeaderboardAction) {
		t.Errorf("ação fora da lista: %v", err)
	}
	for name, reason := range map[string]string{"vazio": "   ", "longo": strings.Repeat("x", 2001), "com NUL": "a\x00b", "UTF-8 inválido": "a\xffb"} {
		_, err := repo.ApplyLeaderboardAction(context.Background(), LeaderboardAction{
			Target: LeaderboardTarget{UserID: u.id}, Action: LeaderboardHide, Reason: reason, Actor: "a",
		})
		if !errors.Is(err, ErrInvalidLeaderboardAction) {
			t.Errorf("motivo %s: %v", name, err)
		}
	}
	if err := moderate(t, repo, LeaderboardTarget{UserID: testUUID(t)}, LeaderboardHide); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("conta que não existe: %v", err)
	}
}
