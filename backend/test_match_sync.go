//go:build ignore

// Script avulso de verificação do sync de partidas.
// Tem `main` próprio, então fica fora do build do pacote — sem a tag acima ele
// colide com o `main` do servidor e quebra `just run-backend`.
// Rode com: go run test_match_sync.go

package main

import (
	"context"
	"fmt"
	"log"
	"github.com/jackc/pgx/v5"
	"os"
	"time"
	"github.com/josecleiton/logn/backend/internal/domain"
)

func main() {
	dbUrl := os.Getenv("DATABASE_URL")
	if dbUrl == "" {
		dbUrl = "postgres://logn_user:logn_password@localhost:5432/logn_db?sslmode=disable"
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dbUrl)
	if err != nil {
		log.Fatalf("Failed to connect to db: %v", err)
	}
	defer conn.Close(ctx)

	repo := domain.NewRepository(conn)
	userID := "00000000-0000-0000-0000-000000000001" // Mock user ID created by init.sql

	// Check initial stats
	var globalXP, bugsFound, dryRuns int
	err = conn.QueryRow(ctx, "SELECT global_xp, bugs_found, dry_runs_completed FROM users WHERE id = $1", userID).Scan(&globalXP, &bugsFound, &dryRuns)
	if err != nil {
		log.Fatalf("Failed to read user stats: %v", err)
	}
	fmt.Printf("Initial Stats - XP: %d, Bugs: %d, DryRuns: %d\n", globalXP, bugsFound, dryRuns)

	// Simulate sync MATCH_ANSWER
	tx, err := conn.Begin(ctx)
	if err != nil {
		log.Fatalf("Failed to start tx: %v", err)
	}

	event1 := domain.GameEvent{
		ID:          "evt_1",
		EventType:   "MATCH_ANSWER",
		PayloadJSON: `{"is_correct": true, "template_type": "SPOT_THE_BUG"}`,
	}
	if err := repo.ProcessEventXP(ctx, tx, userID, event1); err != nil {
		log.Fatalf("Failed to process event1: %v", err)
	}

	event2 := domain.GameEvent{
		ID:          "evt_2",
		EventType:   "MATCH_ANSWER",
		PayloadJSON: `{"is_correct": true, "template_type": "FILL_IN_THE_BLANK"}`,
	}
	if err := repo.ProcessEventXP(ctx, tx, userID, event2); err != nil {
		log.Fatalf("Failed to process event2: %v", err)
	}

	event3 := domain.GameEvent{
		ID:          "evt_3",
		EventType:   "MATCH_END",
		PayloadJSON: `{"solved": 2}`,
	}
	if err := repo.ProcessEventXP(ctx, tx, userID, event3); err != nil {
		log.Fatalf("Failed to process event3: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("Failed to commit tx: %v", err)
	}

	// Check final stats
	err = conn.QueryRow(ctx, "SELECT global_xp, bugs_found, dry_runs_completed FROM users WHERE id = $1", userID).Scan(&globalXP, &bugsFound, &dryRuns)
	if err != nil {
		log.Fatalf("Failed to read user stats after sync: %v", err)
	}
	fmt.Printf("Final Stats - XP: %d, Bugs: %d, DryRuns: %d\n", globalXP, bugsFound, dryRuns)

	if globalXP > 0 && bugsFound > 0 && dryRuns > 0 {
		fmt.Println("SUCCESS! Backend correctly processes MATCH_ANSWER and MATCH_END offline events.")
	} else {
		log.Fatalf("FAILURE: Stats did not increment correctly.")
	}
}
