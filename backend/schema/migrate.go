// Package schema aplica as migrações do banco no boot do servidor.
//
// O `init.sql` só roda quando o Postgres sobe com volume vazio. Toda alteração de
// schema feita depois disso vinha sendo aplicada à mão no banco de quem estava
// desenvolvendo — e não chegava a mais ninguém. Os arquivos de `migrations/` rodam
// em ordem de nome, uma vez cada, com o que já rodou registrado em
// `schema_migrations`.
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
		return nil, fmt.Errorf("criando schema_migrations: %w", err)
	}

	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, fmt.Errorf("lendo migrations: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	var applied []string
	for _, name := range names {
		var exists bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name = $1)`, name,
		).Scan(&exists); err != nil {
			return applied, fmt.Errorf("checando %s: %w", name, err)
		}
		if exists {
			continue
		}

		body, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			return applied, fmt.Errorf("lendo %s: %w", name, err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return applied, fmt.Errorf("abrindo transação para %s: %w", name, err)
		}

		if _, err := tx.Exec(ctx, string(body)); err != nil {
			_ = tx.Rollback(ctx)
			return applied, fmt.Errorf("aplicando %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO schema_migrations (name) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback(ctx)
			return applied, fmt.Errorf("registrando %s: %w", name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return applied, fmt.Errorf("fechando %s: %w", name, err)
		}

		applied = append(applied, name)
	}

	return applied, nil
}
