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
			log.Println("Usando DATABASE_MIGRATION_URL para aplicar as migrações...")
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
			log.Fatalf("Migração falhou: %v\n", err)
		}
		for _, name := range applied {
			log.Printf("migração aplicada: %s", name)
		}
	} else {
		log.Println("Bypassing auto-migrations (RUN_MIGRATIONS != true)")
	}

	if os.Getenv("MIGRATE_ONLY") == "true" {
		log.Println("Migrações concluídas com sucesso. Encerrando (MIGRATE_ONLY=true).")
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
		log.Fatalf("Lista de espera mal configurada: %v", err)
	}
	deps := httpapi.Deps{
		Repo:           repo,
		Mailer:         email.NewMailer(),
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
	}

	if os.Getenv("PURGE_ONLY") == "true" {
		log.Println("Rodando rotina de expurgo (PURGE_ONLY=true)...")
		purged, err := repo.PurgeDeletedAccounts(context.Background())
		if err != nil {
			log.Fatalf("Erro no expurgo depois de %d contas: %v", purged, err)
		}
		pending, err := repo.PurgePendingWaitlist(context.Background())
		if err != nil {
			log.Fatalf("Erro no expurgo da lista de espera: %v", err)
		}
		log.Printf("Expurgo concluído: %d contas e %d inscrições pendentes apagadas. Encerrando.", purged, pending)
		return
	}

	handler, err := httpapi.New(deps)
	if err != nil {
		log.Fatalf("Servidor HTTP não montado: %v", err)
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
