// O `main` é a raiz de composição: lê o ambiente, aborta em produção sem segredo, liga
// banco, migração e dependências, e entrega tudo ao `httpapi` (ADR 0018).
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/josecleiton/logn/backend/internal/domain"
	"github.com/josecleiton/logn/backend/internal/httpapi"
	"github.com/josecleiton/logn/backend/internal/infrastructure/cloudauth"
	"github.com/josecleiton/logn/backend/internal/infrastructure/email"
	"github.com/josecleiton/logn/backend/internal/infrastructure/socialauth"
	"github.com/josecleiton/logn/backend/schema"
)

// Cabeçalho do spec da API, lido pelo `just api-docs` (ADR 0025).
//
//	@title						LogN API
//	@version					1.0
//	@description				API do LogN. Erro de rota que o app lê sai como {"code", "message"}, e o código é contrato.
//	@BasePath					/
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				Access token do login, no formato `Bearer <token>`.
func main() {
	dbUrl := os.Getenv("DATABASE_URL")
	if dbUrl == "" {
		dbUrl = "postgres://logn_user:logn_password@localhost:5432/logn_db?sslmode=disable"
	}

	keys := serverKeysFromEnv()
	domain.JwtSecretKey = jwtSecretFrom(keys)

	config, err := pgxpool.ParseConfig(dbUrl)
	if err != nil {
		log.Fatalf("Invalid DATABASE_URL: %v\n", err)
	}
	config.ConnConfig.ConnectTimeout = 15 * time.Second

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer pool.Close()

	// pool.Ping só verifica o TCP — em bancos serverless (Neon) o compute pode ainda
	// estar acordando enquanto o binário já aceita tráfego, e a primeira query leva 10s
	// esperando o compute estar pronto. Um SELECT real garante que o compute está ativo
	// antes de começar a servir.
	wakeCtx, wakeCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer wakeCancel()
	var dbReady int
	if err := pool.QueryRow(wakeCtx, "SELECT 1").Scan(&dbReady); err != nil {
		log.Fatalf("Database not ready: %v\n", err)
	}

	runMigrations := os.Getenv("RUN_MIGRATIONS") == "true"
	// Cloud Run injeta K_SERVICE. Se não estiver no Cloud Run e a variável não foi definida, roda no local
	if os.Getenv("K_SERVICE") == "" && os.Getenv("RUN_MIGRATIONS") == "" {
		runMigrations = true
	}

	if runMigrations {
		migratePool := pool
		if migrationUrl := os.Getenv("DATABASE_MIGRATION_URL"); migrationUrl != "" {
			log.Println("Using DATABASE_MIGRATION_URL to apply migrations...")
			migrationConfig, err := pgxpool.ParseConfig(migrationUrl)
			if err != nil {
				log.Fatalf("Invalid DATABASE_MIGRATION_URL: %v\n", err)
			}
			migrationConfig.ConnConfig.ConnectTimeout = 15 * time.Second
			migratePool, err = pgxpool.NewWithConfig(context.Background(), migrationConfig)
			if err != nil {
				log.Fatalf("Unable to connect to migration database: %v\n", err)
			}
			defer migratePool.Close()
		}

		applied, err := schema.Migrate(context.Background(), migratePool)
		if err != nil {
			log.Fatalf("Migration failed: %v\n", err)
		}
		for _, name := range applied {
			log.Printf("migration applied: %s", name)
		}
	} else {
		log.Println("Bypassing auto-migrations (RUN_MIGRATIONS != true)")
	}

	if os.Getenv("MIGRATE_ONLY") == "true" {
		log.Println("Migrations completed successfully. Exiting (MIGRATE_ONLY=true).")
		return
	}

	domain.TrackKeySecret = trackKeySecretFrom(keys)

	repo := domain.NewRepository(pool)
	social, revokers := socialVerifiersFromEnv()
	// O GitHub é verificador, revogador e a troca do código ao mesmo tempo. Desligado,
	// fica fora dos três: um *GitHub nulo dentro da interface não seria `nil`.
	var githubExchanger httpapi.GitHubExchanger
	if gh := githubFromEnv(keys, domain.JwtSecretKey); gh != nil {
		social[socialauth.ProviderGitHub] = gh
		revokers[socialauth.ProviderGitHub] = gh
		githubExchanger = gh
	}
	waitlist, err := httpapi.WaitlistConfigFromEnv(os.Getenv)
	if err != nil {
		log.Fatalf("Waitlist misconfigured: %v", err)
	}
	mailer, err := email.NewMailerFromEnv(os.Getenv, os.Getenv("K_SERVICE") != "")
	if err != nil {
		log.Fatalf("SMTP misconfigured: %v", err)
	}
	deps := httpapi.Deps{
		Repo:           repo,
		Mailer:         mailer,
		CloudValidator: cloudauth.NewGoogleValidator(),
		StoreKit:       storeKitValidatorFromEnv(),
		Play:           playFromEnv(),
		Social:         social,
		Revokers:       revokers,
		GitHub:         githubExchanger,
		// Termos e política. No Cloud Run o modo é estrito: documento com marcador de
		// rascunho responde 503, a não ser que LEGAL_ALLOW_DRAFT=true libere a página com a
		// faixa de rascunho — o caso do TestFlight, enquanto o advogado revisa.
		LegalStrict: os.Getenv("K_SERVICE") != "" && os.Getenv("LEGAL_ALLOW_DRAFT") != "true",
		Waitlist:    waitlist,
		OutboxQueue: outboxQueueFromEnv(),
	}

	if os.Getenv("PURGE_ONLY") == "true" {
		log.Println("Running purge routine (PURGE_ONLY=true)...")
		purged, err := repo.PurgeDeletedAccounts(context.Background())
		if err != nil {
			log.Fatalf("Purge failed after %d accounts: %v", purged, err)
		}
		pending, err := repo.PurgePendingWaitlist(context.Background())
		if err != nil {
			log.Fatalf("Waitlist purge failed: %v", err)
		}
		outbox, err := repo.PruneOutbox(context.Background())
		if err != nil {
			log.Fatalf("Outbox purge failed: %v", err)
		}
		log.Printf("Purge completed: %d accounts, %d pending signups and %d outbox e-mails deleted. Exiting.", purged, pending, outbox)
		return
	}

	handler, err := httpapi.New(deps)
	if err != nil {
		log.Fatalf("HTTP server not built: %v", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// ListenAndServe puro não tem timeout nenhum: um cliente que manda o cabeçalho a
	// conta-gotas segura a conexão para sempre.
	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}

	log.Printf("Server starting on :%s...", port)
	if err := httpServer.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
