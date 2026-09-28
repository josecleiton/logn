package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/josecleiton/logn/backend/internal/domain"
	"github.com/josecleiton/logn/backend/internal/infrastructure/cloudauth"
	"github.com/josecleiton/logn/backend/internal/infrastructure/email"
	"github.com/josecleiton/logn/backend/internal/locale"
	"github.com/josecleiton/logn/backend/internal/storekit"
	"github.com/josecleiton/logn/backend/schema"
)

type Server struct {
	repo           *domain.Repository
	mailer         *email.Mailer
	cloudValidator cloudauth.Validator
	storekit       *storekit.Validator
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (s *Server) readyHandler(w http.ResponseWriter, r *http.Request) {
	if err := s.repo.Ping(r.Context()); err != nil {
		http.Error(w, "Database not ready", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ready"))
}

func (s *Server) pingHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "LogN Backend is running!",
	})
}

// authenticate devolve o dono do token do cabeçalho `Authorization: Bearer`.
//
// Responde 401 e devolve `false` quando não há token válido — o chamador só precisa
// desistir.
func (s *Server) authenticate(w http.ResponseWriter, r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		writeError(w, http.StatusUnauthorized, codeUnauthenticated)
		return "", false
	}

	userID, err := domain.UserIDFromAccessToken(strings.TrimPrefix(header, "Bearer "))
	if err != nil {
		writeError(w, http.StatusUnauthorized, codeUnauthenticated)
		return "", false
	}

	// Ensure user exists and has not requested deletion
	if !s.repo.IsUserActive(r.Context(), userID) {
		writeError(w, http.StatusUnauthorized, codeUnauthenticated)
		return "", false
	}

	return userID, true
}

// optionalAccount é o dono do token, quando há token; sem cabeçalho, é o visitante ("").
// Token presente e inválido responde 401 e devolve `false`, para o app renovar a sessão
// em vez de receber a resposta de quem não entrou.
func (s *Server) optionalAccount(w http.ResponseWriter, r *http.Request) (string, bool) {
	if r.Header.Get("Authorization") == "" {
		return "", true
	}
	return s.authenticate(w, r)
}

func (s *Server) syncHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := s.authenticate(w, r)
	if !ok {
		return
	}

	var payload domain.SyncPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}

	// O corpo do pedido não decide de quem é a cadeia. Mandava e o servidor obedecia:
	// dava para escrever eventos na conta de qualquer um.
	payload.UserID = userID

	ctx := r.Context()

	serverLastHash, err := s.repo.GetUserLastHash(ctx, payload.UserID)
	if err != nil {
		log.Printf("sync sem estado: user=%s erro=%v", payload.UserID, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}

	valid, err := domain.ValidateSync(payload, serverLastHash)
	if err != nil {
		if err.Error() == "force_rebase" {
			log.Printf("sync rebase: user=%s eventos=%d topo_servidor=%s primeiro_previous=%s",
				payload.UserID, len(payload.Events), serverLastHash, payload.Events[0].PreviousHash)
			writeRebaseRequired(w, serverLastHash)
			return
		}

		// Sem este log, um sync recusado some: o cliente só vê o número do status e
		// o servidor não conta o motivo a ninguém.
		log.Printf("sync recusado: user=%s eventos=%d motivo=%v", payload.UserID, len(payload.Events), err)
		writeError(w, http.StatusForbidden, codeSyncRejected)
		return
	}

	if !valid {
		log.Printf("sync recusado: user=%s cadeia inválida", payload.UserID)
		writeError(w, http.StatusForbidden, codeSyncRejected)
		return
	}

	// Topo da cadeia depois deste sync. Devolver o topo antigo faria o cliente
	// continuar encadeando a partir de onde o servidor já não está.
	newTop := serverLastHash
	if len(payload.Events) > 0 {
		newTop = payload.Events[len(payload.Events)-1].CurrentHash
		if err := s.repo.InsertSyncEvents(ctx, payload, serverLastHash, newTop); err != nil {
			if errors.Is(err, domain.ErrStaleChain) {
				// Outro sync do mesmo usuário gravou entre a leitura e esta escrita.
				// O topo que ele deixou é o ponto de onde o cliente refaz a fila.
				top, topErr := s.repo.GetUserLastHash(ctx, payload.UserID)
				if topErr == nil {
					log.Printf("sync concorrente: user=%s eventos=%d topo=%s", payload.UserID, len(payload.Events), top)
					writeRebaseRequired(w, top)
					return
				}
				err = topErr
			}
			log.Printf("sync não gravado: user=%s eventos=%d erro=%v", payload.UserID, len(payload.Events), err)
			writeError(w, http.StatusInternalServerError, codeInternal)
			return
		}
		log.Printf("sync ok: user=%s eventos=%d topo=%s", payload.UserID, len(payload.Events), newTop)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "success",
		"events_applied": len(payload.Events),
		"new_top":        newTop,
	})
}

func writeRebaseRequired(w http.ResponseWriter, serverTop string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusConflict)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":           codeRebaseRequired,
		"status":         "rebase_required",
		"server_top":     serverTop,
		"events_applied": 0,
	})
}

// Método filtrado pela rota, como em getNodesHandler: HEAD tem de passar.
func (s *Server) challengesHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.optionalAccount(w, r)
	if !ok {
		return
	}
	lang := locale.Negotiate(r)
	challenges, err := s.repo.GetChallenges(r.Context(), lang, userID)
	if err != nil {
		log.Printf("desafios não lidos: locale=%s erro=%v", lang, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}

	writeAccountContentJSON(w, lang, userID, challenges)
}

// writeContentJSON responde conteúdo da trilha numa língua. `Content-Language` diz ao
// app em que língua o que chegou está — é o que ele grava junto da cópia offline —, e
// `Vary` impede um cache no caminho de servir o espanhol a quem pediu português.
func writeContentJSON(w http.ResponseWriter, lang string, body any) {
	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("Content-Language", lang)
	h.Add("Vary", "Accept-Language")
	h.Set("Cache-Control", "no-cache")
	json.NewEncoder(w).Encode(body)
}

// writeAccountContentJSON responde conteúdo que depende da conta: o visitante recebe o
// de todos, e a conta recebe também o que só ela vê — a trilha comprada, a indisponível
// que ela testa (ADR 0014). Essa não pode ficar num cache no caminho para servir a outra
// pessoa.
func writeAccountContentJSON(w http.ResponseWriter, lang, userID string, body any) {
	if userID == "" {
		writeContentJSON(w, lang, body)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("Content-Language", lang)
	h.Set("Cache-Control", "private, no-store")
	json.NewEncoder(w).Encode(body)
}

func main() {
	dbUrl := os.Getenv("DATABASE_URL")
	if dbUrl == "" {
		dbUrl = "postgres://logn_user:logn_password@localhost:5432/logn_db?sslmode=disable"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		if os.Getenv("K_SERVICE") != "" {
			log.Fatalf("JWT_SECRET is not set. Refusing to start in production with an insecure default.")
		}
		log.Println("WARNING: JWT_SECRET is not set. Using insecure default for development.")
		domain.JwtSecretKey = []byte("my-super-secret-logn-key-for-dev")
	} else {
		domain.JwtSecretKey = []byte(jwtSecret)
	}

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

	domain.TrackKeySecret = trackKeySecretFromEnv()

	repo := domain.NewRepository(pool)
	mailer := email.NewMailer()
	validator := cloudauth.NewGoogleValidator()
	server := &Server{
		repo:           repo,
		mailer:         mailer,
		cloudValidator: validator,
		storekit:       storeKitValidatorFromEnv(),
	}

	if os.Getenv("PURGE_ONLY") == "true" {
		log.Println("Rodando rotina de expurgo (PURGE_ONLY=true)...")
		purged, err := server.repo.PurgeDeletedAccounts(context.Background())
		if err != nil {
			log.Fatalf("Erro no expurgo depois de %d contas: %v", purged, err)
		}
		log.Printf("Expurgo concluído: %d contas apagadas. Encerrando.", purged)
		return
	}

	// Rotas de autenticação passam por um limite por IP. Nenhuma tinha limite, e é
	// por elas que se força senha, se varre OTP e se dispara e-mail. Trinta por
	// minuto folga para quem erra digitando e para vários aparelhos atrás do mesmo NAT.
	authLimiter := newRateLimiter(30, time.Minute)
	auth := func(h http.HandlerFunc) http.HandlerFunc {
		return authLimiter.wrap(limitBody(authBodyLimit, h))
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", server.healthHandler)
	mux.HandleFunc("GET /ready", server.readyHandler)
	mux.HandleFunc("GET /ping", server.pingHandler)
	mux.HandleFunc("POST /api/v1/sync", limitBody(syncBodyLimit, server.syncHandler))
	mux.HandleFunc("GET /api/v1/challenges", server.challengesHandler)
	mux.HandleFunc("POST /api/v1/auth/login", auth(server.loginHandler))
	mux.HandleFunc("POST /api/v1/auth/refresh", auth(server.refreshHandler))
	mux.HandleFunc("POST /api/v1/auth/request-otp", auth(server.requestOTPHandler))
	mux.HandleFunc("POST /api/v1/auth/verify-otp", auth(server.verifyOTPHandler))
	mux.HandleFunc("POST /api/v1/auth/register", auth(server.registerHandler))
	mux.HandleFunc("POST /api/v1/auth/reset-password", auth(server.resetPasswordHandler))
	mux.HandleFunc("POST /api/v1/users/me/delete", auth(server.deleteAccountHandler))
	mux.HandleFunc("POST /api/v1/purchases", auth(server.purchaseHandler))
	mux.HandleFunc("POST /api/v1/purchases/restore", auth(server.restorePurchaseHandler))
	// Catálogo, licença e pacote têm balde próprio: cada abertura revalida as trilhas da
	// conta, e dividir o balde do login deixava o jogador atrás de NAT sem conseguir entrar.
	trackLimiter := newRateLimiter(60, time.Minute)
	tracked := func(h http.HandlerFunc) http.HandlerFunc {
		return trackLimiter.wrap(limitBody(authBodyLimit, h))
	}
	mux.HandleFunc("GET /api/v1/tracks", tracked(server.tracksHandler))
	mux.HandleFunc("GET /api/v1/tracks/{id}/license", tracked(server.trackLicenseHandler))
	mux.HandleFunc("GET /api/v1/tracks/{id}/package", tracked(server.trackPackageHandler))

	mux.HandleFunc("POST /api/v1/appstore/notifications",
		server.appStoreNotificationRoute(newRateLimiter(120, time.Minute)))

	mux.HandleFunc("GET /api/v1/nodes", server.getNodesHandler)
	mux.HandleFunc("GET /api/v1/progress", server.getUserProgressHandler)
	mux.HandleFunc("POST /api/v1/internal/purge", server.purgeHandler)

	// Termos e política. No Cloud Run o modo é estrito: documento com marcador de
	// rascunho responde 503, a não ser que LEGAL_ALLOW_DRAFT=true libere a página com a
	// faixa de rascunho — o caso do TestFlight, enquanto o advogado revisa.
	legalStrict := os.Getenv("K_SERVICE") != "" && os.Getenv("LEGAL_ALLOW_DRAFT") != "true"
	registerLegalRoutes(mux, legalStore{server.repo}, legalStrict)

	mux.HandleFunc("GET /api/v1/legal/current", server.currentLegalVersionsHandler)
	mux.HandleFunc("GET /api/v1/legal/pending", auth(server.pendingLegalHandler))
	mux.HandleFunc("POST /api/v1/legal/accept", auth(server.acceptLegalHandler))

	// Sondas do Cloud Run e a rota interna do Cloud Scheduler chegam direto do Google,
	// nunca pelo proxy na frente — ficam fora da checagem de origem.
	//
	// A notificação da App Store também: é servidor chamando servidor, e o Bot Fight
	// Mode da borda, que não aceita exceção, desafiaria a Apple com JS e o reembolso
	// nunca chegaria. Ela é cadastrada na URL .run.app, como o Scheduler, e a defesa é a
	// assinatura contra a raiz fixa (ADR 0013), com teto de corpo e de ritmo.
	originExempt := func(r *http.Request) bool {
		switch r.URL.Path {
		case "/health", "/ready", "/ping", "/api/v1/appstore/notifications":
			return true
		}
		return strings.HasPrefix(r.URL.Path, "/api/v1/internal/")
	}

	var handler http.Handler = mux
	if os.Getenv("K_SERVICE") != "" {
		origin, err := newOriginVerifierFromEnv()
		if err != nil {
			log.Fatalf("Verificação de origem não configurada: %v", err)
		}
		handler = origin.wrap(handler, originExempt)
	} else if os.Getenv("ORIGIN_TRUSTED_CIDRS") != "" || os.Getenv("ORIGIN_SHARED_SECRET") != "" {
		origin, err := newOriginVerifierFromEnv()
		if err != nil {
			log.Fatalf("Verificação de origem mal configurada: %v", err)
		}
		handler = origin.wrap(handler, originExempt)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// ListenAndServe puro não tem timeout nenhum: um cliente que manda o cabeçalho a
	// conta-gotas segura a conexão para sempre.
	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           withGzip(handler),
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
