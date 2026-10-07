// Package schema aplica as migrações do banco no boot do servidor.
//
// Os arquivos de `migrations/` são o schema inteiro, a partir da 0000 — banco vazio
// sai daqui pronto, sem script de init à parte. Rodam em ordem de nome, uma vez
// cada, com o que já rodou registrado em `schema_migrations`.
//
// Convenção: `NNNN_descricao.sql`, numeração crescente, nunca editar um arquivo já
// aplicado — escreva o próximo.
package schema

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migrate aplica o que falta e devolve os nomes aplicados nesta execução.
//
// Cada arquivo roda dentro de uma transação: ou entra inteiro e fica registrado, ou
// não entra.
func Migrate(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			name       VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`)
	if err != nil {
		return nil, fmt.Errorf("creating schema_migrations: %w", err)
	}

	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, fmt.Errorf("reading migrations: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	// Uma query traz todas as migrações já aplicadas; a abordagem anterior fazia um
	// SELECT EXISTS por arquivo (21 round trips para conferir que nada mudou).
	rows, err := pool.Query(ctx, `SELECT name FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("reading schema_migrations: %w", err)
	}
	alreadyApplied := make(map[string]bool, len(names))
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			rows.Close()
			return nil, fmt.Errorf("reading migration name: %w", err)
		}
		alreadyApplied[n] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating schema_migrations: %w", err)
	}

	var applied []string
	for _, name := range names {
		if alreadyApplied[name] {
			continue
		}

		body, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			return applied, fmt.Errorf("reading %s: %w", name, err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return applied, fmt.Errorf("opening transaction for %s: %w", name, err)
		}

		if _, err := tx.Exec(ctx, string(body)); err != nil {
			_ = tx.Rollback(ctx)
			return applied, fmt.Errorf("applying %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO schema_migrations (name) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback(ctx)
			return applied, fmt.Errorf("recording %s: %w", name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return applied, fmt.Errorf("committing %s: %w", name, err)
		}

		applied = append(applied, name)
	}

	return applied, nil
}
