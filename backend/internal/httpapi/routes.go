package httpapi

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/josecleiton/logn/backend/internal/domain"
	"github.com/josecleiton/logn/backend/internal/infrastructure/cloudauth"
	"github.com/josecleiton/logn/backend/internal/infrastructure/email"
	"github.com/josecleiton/logn/backend/internal/infrastructure/socialauth"
	"github.com/josecleiton/logn/backend/internal/storekit"
)

// Deps é o que o servidor recebe pronto de quem o monta.
type Deps struct {
	Repo           *domain.Repository
	Mailer         *email.Mailer
	CloudValidator cloudauth.Validator
	StoreKit       *storekit.Validator
	// Um verificador por provedor de login ligado. Provedor fora do mapa está desligado.
	Social map[string]socialauth.Verifier
	// Provedores que revogam o acesso na exclusão da conta (a Apple, o GitHub).
	Revokers map[string]socialauth.Revoker
	// Troca o código do GitHub pelo bilhete (ADR 0019). Nulo com o GitHub desligado.
	GitHub GitHubExchanger
	// Documento legal com marcador de rascunho responde 503 em vez da página.
	LegalStrict bool
}

// New monta as rotas e o middleware em volta delas.
//
// Falha quando a verificação de origem é obrigatória (Cloud Run) ou foi pedida pelo
// ambiente e está mal configurada: quem chama não deve subir o servidor sem ela.
func New(d Deps) (http.Handler, error) {
	server := &Server{
		repo:           d.Repo,
		mailer:         d.Mailer,
		cloudValidator: d.CloudValidator,
		storekit:       d.StoreKit,
		social:         d.Social,
		revokers:       d.Revokers,
		github:         d.GitHub,
	}
	if d.Mailer != nil {
		server.licenseNotifier = d.Mailer
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
	mux.HandleFunc("POST /api/v1/auth/social", auth(server.socialLoginHandler))
	mux.HandleFunc("POST /api/v1/auth/github/exchange", auth(server.githubExchangeHandler))
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
	// Revogação manual de licença e resposta à contestação (ADR 0021). Quem chama é uma
	// pessoa pelo `just revoke`; dez por minuto sobra.
	internalLimiter := newRateLimiter(10, time.Minute)
	internal := func(h http.HandlerFunc) http.HandlerFunc {
		return internalLimiter.wrap(limitBody(licenseActionBodyLimit, h))
	}
	mux.HandleFunc("POST /api/v1/internal/licenses/revoke", internal(server.revokeLicenseHandler))
	mux.HandleFunc("POST /api/v1/internal/licenses/appeal", internal(server.appealLicenseHandler))

	registerLegalRoutes(mux, legalStore{server.repo}, d.LegalStrict)

	mux.HandleFunc("GET /api/v1/legal/current", server.currentLegalVersionsHandler)
	// Pendência e aceite têm balde próprio, como as trilhas: toda abertura pergunta pelos
	// termos, e dividir o balde do login gastava a entrada de quem está atrás de NAT e
	// fazia um 429 pular o bloqueio (ADR 0020).
	legalLimiter := newRateLimiter(60, time.Minute)
	legalRoute := func(h http.HandlerFunc) http.HandlerFunc {
		return legalLimiter.wrap(limitBody(authBodyLimit, h))
	}
	mux.HandleFunc("GET /api/v1/legal/pending", legalRoute(server.pendingLegalHandler))
	mux.HandleFunc("POST /api/v1/legal/accept", legalRoute(server.acceptLegalHandler))

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
			return nil, fmt.Errorf("verificação de origem não configurada: %w", err)
		}
		handler = origin.wrap(handler, originExempt)
	} else if os.Getenv("ORIGIN_TRUSTED_CIDRS") != "" || os.Getenv("ORIGIN_SHARED_SECRET") != "" {
		origin, err := newOriginVerifierFromEnv()
		if err != nil {
			return nil, fmt.Errorf("verificação de origem mal configurada: %w", err)
		}
		handler = origin.wrap(handler, originExempt)
	}

	return withGzip(handler), nil
}
