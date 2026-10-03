package domain

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// As migrações 0070 e 0071 recalculam `free_xp` com o 50 escrito no SQL. Se a
// constante mudar, as duas contas passam a mentir: este teste obriga a revê-las.
func TestXPPerAcceptedAnswerIsTheOneTheMigrationsUse(t *testing.T) {
	if XPPerAcceptedAnswer != 50 {
		t.Fatalf("XPPerAcceptedAnswer = %d; as migrações 0070 e 0071 usam 50", XPPerAcceptedAnswer)
	}
}

func TestDrawAnonNumberStaysInRange(t *testing.T) {
	for _, c := range []struct{ digits, low, high int }{{4, 1000, 9999}, {5, 10000, 99999}} {
		for i := 0; i < 500; i++ {
			n, err := drawAnonNumber(c.digits)
			if err != nil {
				t.Fatal(err)
			}
			if n < c.low || n > c.high {
				t.Fatalf("%d dígitos sorteou %d", c.digits, n)
			}
		}
	}
}

// Troca o sorteio por uma sequência fixa e devolve os dígitos pedidos em cada chamada.
func scriptAnonDraw(t *testing.T, numbers ...int) *[]int {
	t.Helper()
	asked := &[]int{}
	old := anonDraw
	i := 0
	anonDraw = func(digits int) (int, error) {
		*asked = append(*asked, digits)
		n := numbers[i]
		if i < len(numbers)-1 {
			i++
		}
		return n, nil
	}
	t.Cleanup(func() { anonDraw = old })
	return asked
}

func anonNumberOf(t *testing.T, repo *Repository, userID string) int {
	t.Helper()
	var n int
	if err := repo.db.QueryRow(context.Background(), `SELECT anon_number FROM users WHERE id = $1`, userID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func createTestUser(t *testing.T, repo *Repository) string {
	t.Helper()
	ctx := context.Background()
	id, err := repo.CreateUser(ctx, "placar-"+testUUID(t)[:12]+"@example.com", "", true, "", nil, ClientInfo{})
	if err != nil {
		t.Fatalf("cadastro: %v", err)
	}
	t.Cleanup(func() { repo.db.Exec(ctx, `DELETE FROM users WHERE id = $1`, id) })
	return id
}

// Um número fora da faixa que o servidor sorteia, para o teste não colidir com conta de
// verdade do banco local.
func farAnonNumber(t *testing.T) int {
	t.Helper()
	n, err := drawAnonNumber(8)
	if err != nil {
		t.Fatal(err)
	}
	return 100_000_000 + n
}

func TestCreateUserRedrawsOnAnonCollision(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)

	taken := farAnonNumber(t)
	scriptAnonDraw(t, taken)
	first := createTestUser(t, repo)
	if got := anonNumberOf(t, repo, first); got != taken {
		t.Fatalf("primeira conta com %d, esperava %d", got, taken)
	}

	free := taken + 1
	scriptAnonDraw(t, taken, free)
	second := createTestUser(t, repo)
	if got := anonNumberOf(t, repo, second); got != free {
		t.Fatalf("o número já usado tinha de ser sorteado de novo: ficou %d, esperava %d", got, free)
	}
}

func TestCreateUserWidensAfterRepeatedCollisions(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)

	taken := farAnonNumber(t)
	scriptAnonDraw(t, taken)
	createTestUser(t, repo)

	free := taken + 1
	asked := scriptAnonDraw(t, taken, taken, taken, free)
	createTestUser(t, repo)
	want := []int{4, 4, 4, 5}
	if len(*asked) != len(want) {
		t.Fatalf("sorteios pedidos: %v, esperava %v", *asked, want)
	}
	for i := range want {
		if (*asked)[i] != want[i] {
			t.Fatalf("dígitos pedidos: %v, esperava %v", *asked, want)
		}
	}
}

// Todos os sorteios colidindo: o cadastro falha com erro, sem conta pela metade.
func TestCreateUserGivesUpAfterMaxDraws(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()

	taken := farAnonNumber(t)
	scriptAnonDraw(t, taken)
	createTestUser(t, repo)

	asked := scriptAnonDraw(t, taken)
	email := "placar-cheio-" + testUUID(t)[:12] + "@example.com"
	if _, err := repo.CreateUser(ctx, email, "", true, "", nil, ClientInfo{}); err == nil {
		t.Fatal("o cadastro passou com todos os sorteios colidindo")
	}
	if len(*asked) != anonMaxDraws {
		t.Fatalf("sorteou %d vezes, esperava %d", len(*asked), anonMaxDraws)
	}
	var n int
	conn.QueryRow(ctx, `SELECT count(*) FROM users WHERE email = $1`, email).Scan(&n)
	if n != 0 {
		t.Fatal("a conta ficou gravada sem número")
	}
}

// O ON CONFLICT cobre só o número: e-mail repetido continua sendo 23505, que é como os
// handlers reconhecem "e-mail já tem conta".
func TestCreateUserStillReportsEmailTaken(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()

	email := "placar-dup-" + testUUID(t)[:12] + "@example.com"
	id, err := repo.CreateUser(ctx, email, "", true, "", nil, ClientInfo{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Exec(ctx, `DELETE FROM users WHERE id = $1`, id) })

	_, err = repo.CreateUser(ctx, email, "", true, "", nil, ClientInfo{})
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("e-mail repetido devolveu %v, esperava 23505", err)
	}
}

func freeXPOf(t *testing.T, repo *Repository, userID string) (int, bool) {
	t.Helper()
	var xp int
	var reached bool
	if err := repo.db.QueryRow(context.Background(),
		`SELECT free_xp, free_xp_reached_at IS NOT NULL FROM users WHERE id = $1`, userID).Scan(&xp, &reached); err != nil {
		t.Fatal(err)
	}
	return xp, reached
}

// Desafio de trilha paga, amostra incluída, anda o global e não o placar.
func TestPaidTrackChallengeMovesGlobalXPButNotFreeXP(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	p := seedPaidTrack(t, conn)
	ctx := context.Background()
	a := seedUser(t, conn)

	processAnswer(t, repo, conn, a, answer(t, p.sampleCh, p.sampleNode))
	if _, err := repo.GrantEntitlement(ctx, grant(p, a, a, "test-"+testUUID(t)[:12], false)); err != nil {
		t.Fatal(err)
	}
	processAnswer(t, repo, conn, a, answer(t, p.closedCh, p.closed))

	if xp := xpOf(t, conn, a); xp != 2*XPPerAcceptedAnswer {
		t.Fatalf("global_xp = %d, esperava %d", xp, 2*XPPerAcceptedAnswer)
	}
	if xp, reached := freeXPOf(t, repo, a); xp != 0 || reached {
		t.Fatalf("free_xp = %d (reached %v) com XP só de trilha paga", xp, reached)
	}
}
