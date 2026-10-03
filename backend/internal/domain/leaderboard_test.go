package domain

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSettleLeaderboardClosedBelowTen(t *testing.T) {
	rank := 1
	board := Leaderboard{Rows: []LeaderboardRow{{Rank: 1, IsMe: true}}}
	settleLeaderboard(&board, 7, &rank, LeaderboardOpenAt)
	if board.Open || board.Missing != 3 {
		t.Fatalf("7 visíveis: open=%v missing=%d, esperava fechado faltando 3", board.Open, board.Missing)
	}
	if len(board.Rows) != 0 || board.Me.Rank != nil {
		t.Fatalf("fechado deixou sair posição: rows=%v me.rank=%v", board.Rows, board.Me.Rank)
	}
}

func TestSettleLeaderboardOpensAtTen(t *testing.T) {
	rank := 4
	board := Leaderboard{Rows: []LeaderboardRow{{Rank: 4, IsMe: true}}}
	settleLeaderboard(&board, LeaderboardOpenAt, &rank, LeaderboardOpenAt)
	if !board.Open || board.Missing != 0 || board.Me.Rank == nil || *board.Me.Rank != 4 || len(board.Rows) != 1 {
		t.Fatalf("10 visíveis: %+v", board)
	}
}

// Um limiar que o banco nunca alcança deixa ver, contra o banco de verdade, o placar
// fechado: nenhuma posição sai, nem a do próprio jogador.
const unreachableOpenAt = 1_000_000_000

func TestClosedBoardLetsNoRankOut(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	me := seedBoardUser(t, conn, 990_000_000, time.Now())

	board, err := repo.leaderboardOpeningAt(context.Background(), me.id, unreachableOpenAt)
	if err != nil {
		t.Fatal(err)
	}
	if board.Open || board.Missing <= 0 || board.Threshold != unreachableOpenAt {
		t.Fatalf("open=%v missing=%d threshold=%d", board.Open, board.Missing, board.Threshold)
	}
	if len(board.Rows) != 0 || board.Me.Rank != nil {
		t.Fatalf("fechado deixou sair posição: rows=%d me.rank=%v", len(board.Rows), board.Me.Rank)
	}
	if board.Me.XP != 990_000_000 || board.Me.AnonNumber != me.anon {
		t.Fatalf("a linha do jogador continua vindo, sem posição: %+v", board.Me)
	}
}

// Conta oculta e conta com exclusão pedida não contam para abrir o placar, e uma conta
// com XP conta.
func TestHiddenAndDeletedDoNotCountTowardsOpening(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()
	me := seedBoardUser(t, conn, 990_000_000, time.Now())

	missing := func() int {
		t.Helper()
		board, err := repo.leaderboardOpeningAt(ctx, me.id, unreachableOpenAt)
		if err != nil {
			t.Fatal(err)
		}
		return board.Missing
	}
	before := missing()
	hidden := seedBoardUser(t, conn, 990_000_001, time.Now())
	deleted := seedBoardUser(t, conn, 990_000_002, time.Now())
	if got := missing(); got != before-2 {
		t.Fatalf("duas contas com XP: faltavam %d, agora %d", before, got)
	}
	conn.Exec(ctx, `UPDATE users SET leaderboard_hidden = true WHERE id = $1`, hidden.id)
	conn.Exec(ctx, `UPDATE users SET deletion_requested_at = CURRENT_TIMESTAMP WHERE id = $1`, deleted.id)
	if got := missing(); got != before {
		t.Fatalf("oculta e com exclusão pedida ainda contam: faltavam %d, agora %d", before, got)
	}
}

// boardUser cria uma conta de teste com XP da trilha gratuita escrito direto, como se
// os desafios já tivessem pago. O XP é alto para ficar acima das contas do banco local.
type boardUser struct {
	id   string
	anon int
}

func seedBoardUser(t *testing.T, conn *pgxpool.Pool, xp int, reached time.Time) boardUser {
	t.Helper()
	ctx := context.Background()
	u := boardUser{id: testUUID(t), anon: farAnonNumber(t)}
	var reachedAt *time.Time
	if xp > 0 {
		reachedAt = &reached
	}
	if _, err := conn.Exec(ctx, `
		INSERT INTO users (id, email, anon_number, global_xp, free_xp, free_xp_reached_at)
		VALUES ($1, $2, $3, $4, $4, $5)`,
		u.id, "placar-"+u.id[:8]+"@example.com", u.anon, xp, reachedAt); err != nil {
		t.Fatalf("conta do placar: %v", err)
	}
	t.Cleanup(func() { conn.Exec(ctx, `DELETE FROM users WHERE id = $1`, u.id) })
	return u
}

// Dez contas de XP altíssimo garantem o placar aberto, qualquer que seja o banco local.
func seedOpenBoard(t *testing.T, conn *pgxpool.Pool) []boardUser {
	t.Helper()
	users := make([]boardUser, 0, LeaderboardOpenAt)
	base := time.Now().Add(-time.Hour)
	for i := 0; i < LeaderboardOpenAt; i++ {
		users = append(users, seedBoardUser(t, conn, 900_000_000+i, base))
	}
	return users
}

func rowFor(board Leaderboard, anon int) *LeaderboardRow {
	for i := range board.Rows {
		if board.Rows[i].AnonNumber == anon {
			return &board.Rows[i]
		}
	}
	return nil
}

func TestLeaderboardTieBreakIsWhoGotThereFirst(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	seedOpenBoard(t, conn)

	now := time.Now()
	late := seedBoardUser(t, conn, 950_000_000, now.Add(-time.Minute))
	early := seedBoardUser(t, conn, 950_000_000, now.Add(-time.Hour))

	board, err := repo.GetLeaderboard(context.Background(), late.id)
	if err != nil {
		t.Fatal(err)
	}
	e, l := rowFor(board, early.anon), rowFor(board, late.anon)
	if e == nil || l == nil {
		t.Fatalf("as duas contas empatadas tinham de estar na lista: %+v", board.Rows)
	}
	if e.Rank >= l.Rank {
		t.Fatalf("empate: quem chegou antes ficou em %d e quem chegou depois em %d", e.Rank, l.Rank)
	}
	if !l.IsMe || e.IsMe {
		t.Fatal("is_me marcou a linha errada")
	}
	if board.Me.Rank == nil || *board.Me.Rank != l.Rank {
		t.Fatalf("me.rank = %v, esperava %d", board.Me.Rank, l.Rank)
	}
}

func TestHiddenDeletedAndZeroXPAreNotInRows(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()
	open := seedOpenBoard(t, conn)

	hidden := seedBoardUser(t, conn, 960_000_000, time.Now())
	deleted := seedBoardUser(t, conn, 960_000_001, time.Now())
	zero := seedBoardUser(t, conn, 0, time.Time{})
	conn.Exec(ctx, `UPDATE users SET leaderboard_hidden = true WHERE id = $1`, hidden.id)
	conn.Exec(ctx, `UPDATE users SET deletion_requested_at = CURRENT_TIMESTAMP WHERE id = $1`, deleted.id)

	board, err := repo.GetLeaderboard(ctx, open[0].id)
	if err != nil {
		t.Fatal(err)
	}
	for name, u := range map[string]boardUser{"oculta": hidden, "com exclusão pedida": deleted, "com XP zero": zero} {
		if rowFor(board, u.anon) != nil {
			t.Errorf("conta %s apareceu no placar", name)
		}
	}
}

func TestHiddenSeesOwnRowWithoutRank(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()
	seedOpenBoard(t, conn)

	hidden := seedBoardUser(t, conn, 970_000_000, time.Now())
	conn.Exec(ctx, `UPDATE users SET leaderboard_hidden = true WHERE id = $1`, hidden.id)

	board, err := repo.GetLeaderboard(ctx, hidden.id)
	if err != nil {
		t.Fatal(err)
	}
	if !board.Me.Hidden || board.Me.Rank != nil || board.Me.XP != 970_000_000 || board.Me.AnonNumber != hidden.anon {
		t.Fatalf("a conta oculta vê a própria linha sem posição: %+v", board.Me)
	}
}

func TestMeOutsideTop100(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)

	base := time.Now().Add(-time.Hour)
	for i := 0; i <= LeaderboardTop; i++ {
		seedBoardUser(t, conn, 980_000_000+i, base)
	}
	me := seedBoardUser(t, conn, 970_000_000, base)

	board, err := repo.GetLeaderboard(context.Background(), me.id)
	if err != nil {
		t.Fatal(err)
	}
	if len(board.Rows) != LeaderboardTop {
		t.Fatalf("a lista tem %d linhas, esperava %d", len(board.Rows), LeaderboardTop)
	}
	if rowFor(board, me.anon) != nil {
		t.Fatal("a linha do jogador fora do top entrou na lista")
	}
	if board.Me.Rank == nil || *board.Me.Rank <= LeaderboardTop {
		t.Fatalf("me.rank = %v, esperava a posição depois do top", board.Me.Rank)
	}
}
