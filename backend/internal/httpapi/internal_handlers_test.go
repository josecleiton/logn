package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/josecleiton/logn/backend/internal/domain"
	"github.com/josecleiton/logn/backend/internal/infrastructure/cloudauth"
	"github.com/josecleiton/logn/backend/internal/infrastructure/email"
)

const (
	testAudience         = "https://my-app"
	testSchedulerAccount = "scheduler@example-project.iam.gserviceaccount.com"
	testAdminAccount     = "logn-admin@example-project.iam.gserviceaccount.com"
)

// mockCloudValidator aceita "Bearer <e-mail>" e diz que o token é daquela conta. Um
// token assim passou pela assinatura, emissor e audiência: o que sobra para a rota
// conferir é a conta.
type mockCloudValidator struct {
	shouldFail bool
}

func (m *mockCloudValidator) ValidateToken(ctx context.Context, authHeader string, expectedAudience string) (cloudauth.Caller, error) {
	if m.shouldFail || !strings.HasPrefix(authHeader, "Bearer ") {
		return cloudauth.Caller{}, errors.New("invalid token")
	}
	return cloudauth.Caller{Email: strings.TrimPrefix(authHeader, "Bearer ")}, nil
}

func internalEnv(t *testing.T, audience, scheduler, admin string) {
	t.Setenv("CLOUD_SCHEDULER_AUDIENCE", audience)
	t.Setenv(envSchedulerAccount, scheduler)
	t.Setenv(envAdminAccount, admin)
}

func TestPurgeHandler(t *testing.T) {
	pool := setupTestDB(t)
	repo := domain.NewRepository(pool)

	tests := []struct {
		name           string
		method         string
		audience       string
		scheduler      string
		authHeader     string
		shouldFail     bool
		expectedStatus int
	}{
		{"Method Not Allowed", http.MethodGet, testAudience, testSchedulerAccount, "Bearer " + testSchedulerAccount, false, http.StatusMethodNotAllowed},
		{"Endpoint Disabled (No Audience Config)", http.MethodPost, "", testSchedulerAccount, "Bearer " + testSchedulerAccount, false, http.StatusForbidden},
		{"Endpoint Disabled (No Service Account Config)", http.MethodPost, testAudience, "", "Bearer " + testSchedulerAccount, false, http.StatusForbidden},
		{"Unauthorized (No Header)", http.MethodPost, testAudience, testSchedulerAccount, "", false, http.StatusUnauthorized},
		{"Unauthorized (Invalid Token)", http.MethodPost, testAudience, testSchedulerAccount, "Bearer " + testSchedulerAccount, true, http.StatusUnauthorized},
		// O token é válido e do Google, mas de outra conta de serviço: qualquer projeto
		// consegue um com esta audiência.
		{"Forbidden (Another Service Account)", http.MethodPost, testAudience, testSchedulerAccount, "Bearer attacker@other-project.iam.gserviceaccount.com", false, http.StatusForbidden},
		{"Forbidden (Admin Account Cannot Purge)", http.MethodPost, testAudience, testSchedulerAccount, "Bearer " + testAdminAccount, false, http.StatusForbidden},
		{"Authorized", http.MethodPost, testAudience, testSchedulerAccount, "Bearer " + testSchedulerAccount, false, http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			internalEnv(t, tc.audience, tc.scheduler, testAdminAccount)

			server := &Server{
				repo:           repo,
				cloudValidator: &mockCloudValidator{shouldFail: tc.shouldFail},
			}

			req, _ := http.NewRequest(tc.method, "/api/v1/internal/purge", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rr := httptest.NewRecorder()

			server.purgeHandler(rr, req)

			if status := rr.Code; status != tc.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tc.expectedStatus)
			}
		})
	}
}

// fakeNotifier guarda os avisos em vez de mandar.
type fakeNotifier struct {
	sent []sentNotice
	fail bool
}

type sentNotice struct {
	to, lang, track, reason string
	kind                    email.LicenseNoticeKind
}

func (n *fakeNotifier) SendLicenseNotice(to string, kind email.LicenseNoticeKind, lang, track, reason string) error {
	if n.fail {
		return errors.New("smtp fora")
	}
	n.sent = append(n.sent, sentNotice{to: to, kind: kind, lang: lang, track: track, reason: reason})
	return nil
}

func licenseCall(t *testing.T, s *Server, handler func(*Server) http.HandlerFunc, caller string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/internal/licenses", bytes.NewReader(raw))
	if caller != "" {
		req.Header.Set("Authorization", "Bearer "+caller)
	}
	rr := httptest.NewRecorder()
	handler(s)(rr, req)
	return rr
}

func revokeRoute(s *Server) http.HandlerFunc { return s.revokeLicenseHandler }
func appealRoute(s *Server) http.HandlerFunc { return s.appealLicenseHandler }

// Uma compra de Alice, pronta para a revogação manual.
func boughtByAlice(t *testing.T) (*storeFixture, *fakeNotifier) {
	t.Helper()
	f := newStoreFixture(t)
	txID := "test-" + newTestUUID(t)[:12]
	expect(t, f.post(t, f.server.purchaseHandler, "/api/v1/purchases", f.alice,
		map[string]string{"jws": f.purchaseJWS(t, txID, f.alice)}), http.StatusOK, "")
	n := &fakeNotifier{}
	f.server.licenseNotifier = n
	f.server.cloudValidator = &mockCloudValidator{}
	internalEnv(t, testAudience, testSchedulerAccount, testAdminAccount)
	return f, n
}

func TestOnlyTheAdminAccountRevokes(t *testing.T) {
	f, n := boughtByAlice(t)
	body := map[string]string{"user_id": f.alice, "track_id": f.trackID, "reason": "redistribution", "evidence": "enunciados num fórum"}

	for name, caller := range map[string]string{
		"sem token":          "",
		"outra conta":        "attacker@other-project.iam.gserviceaccount.com",
		"conta do Scheduler": testSchedulerAccount,
		"e-mail parecido":    testAdminAccount + ".evil",
	} {
		rr := licenseCall(t, f.server, revokeRoute, caller, body)
		if rr.Code != http.StatusUnauthorized && rr.Code != http.StatusForbidden {
			t.Errorf("%s: got %d", name, rr.Code)
		}
	}
	// Sem a conta de administração configurada, a rota fecha.
	t.Setenv(envAdminAccount, "")
	expect(t, licenseCall(t, f.server, revokeRoute, testAdminAccount, body), http.StatusForbidden, "")
	if len(n.sent) != 0 {
		t.Fatalf("aviso saiu sem revogação: %+v", n.sent)
	}
	expect(t, f.license(t, f.alice, newTestUUID(t)), http.StatusOK, "")
}

func TestARevocationAndItsAppealNotifyAtEveryStep(t *testing.T) {
	f, n := boughtByAlice(t)

	rr := licenseCall(t, f.server, revokeRoute, testAdminAccount,
		map[string]string{"user_id": f.alice, "track_id": f.trackID, "reason": "account_sharing", "evidence": "dois aparelhos ativos na mesma hora"})
	expect(t, rr, http.StatusOK, "")
	expect(t, f.license(t, f.alice, newTestUUID(t)), http.StatusForbidden, codeEntitlementRequired)
	if len(n.sent) != 1 || n.sent[0].kind != email.LicenseRevoked || n.sent[0].reason != "account_sharing" ||
		n.sent[0].track != "Trilha T" || !strings.HasSuffix(n.sent[0].to, "@example.com") {
		t.Fatalf("aviso da revogação: %+v", n.sent)
	}

	// A contestação chegou: a licença volta enquanto analisamos.
	expect(t, licenseCall(t, f.server, appealRoute, testAdminAccount,
		map[string]string{"user_id": f.alice, "track_id": f.trackID, "outcome": "review", "evidence": "contestação recebida"}),
		http.StatusOK, "")
	expect(t, f.license(t, f.alice, newTestUUID(t)), http.StatusOK, "")
	if len(n.sent) != 2 || n.sent[1].kind != email.LicenseUnderReview {
		t.Fatalf("aviso da análise: %+v", n.sent)
	}
	// Uma revogação nova com a análise aberta não passa: a decisão sai pela contestação.
	expect(t, licenseCall(t, f.server, revokeRoute, testAdminAccount,
		map[string]string{"user_id": f.alice, "track_id": f.trackID, "reason": "redistribution", "evidence": "e"}),
		http.StatusConflict, codeLicenseAppealOutOfOrder)

	// Recusada: a licença sai de novo, com o aviso da recusa.
	expect(t, licenseCall(t, f.server, appealRoute, testAdminAccount,
		map[string]string{"user_id": f.alice, "track_id": f.trackID, "outcome": "rejected", "evidence": "análise concluída"}),
		http.StatusOK, "")
	expect(t, f.license(t, f.alice, newTestUUID(t)), http.StatusForbidden, codeEntitlementRequired)
	if len(n.sent) != 3 || n.sent[2].kind != email.LicenseRevokedAfterReview || n.sent[2].reason != "account_sharing" {
		t.Fatalf("aviso da recusa: %+v", n.sent)
	}

	var actor string
	f.conn.QueryRow(context.Background(), `SELECT actor FROM license_actions WHERE user_id = $1 ORDER BY id DESC LIMIT 1`, f.alice).Scan(&actor)
	if actor != testAdminAccount {
		t.Fatalf("autor gravado %q, esperava a conta do token", actor)
	}
}

func TestLicenseRoutesRefuseBadRequests(t *testing.T) {
	f, n := boughtByAlice(t)

	for _, body := range map[string]any{
		"user_id que não é uuid": map[string]string{"user_id": "x", "track_id": f.trackID, "reason": "redistribution", "evidence": "e"},
		"motivo da loja":         map[string]string{"user_id": f.alice, "track_id": f.trackID, "reason": "refund", "evidence": "e"},
		"sem evidência":          map[string]string{"user_id": f.alice, "track_id": f.trackID, "reason": "redistribution"},
		// O autor vem do token, nunca do corpo.
		"campo a mais": map[string]string{"user_id": f.alice, "track_id": f.trackID, "reason": "redistribution", "evidence": "e", "actor": "eu"},
	} {
		expect(t, licenseCall(t, f.server, revokeRoute, testAdminAccount, body), http.StatusBadRequest, codeInvalidRequest)
	}
	expect(t, licenseCall(t, f.server, revokeRoute, testAdminAccount,
		map[string]string{"user_id": f.bob, "track_id": f.trackID, "reason": "redistribution", "evidence": "e"}),
		http.StatusConflict, codeLicenseNotActive)
	expect(t, licenseCall(t, f.server, appealRoute, testAdminAccount,
		map[string]string{"user_id": f.alice, "track_id": f.trackID, "outcome": "accepted", "evidence": "e"}),
		http.StatusConflict, codeLicenseNotRevoked)
	expect(t, licenseCall(t, f.server, appealRoute, testAdminAccount,
		map[string]string{"user_id": f.alice, "track_id": f.trackID, "reason": "redistribution", "outcome": "accepted", "evidence": "e"}),
		http.StatusBadRequest, codeInvalidRequest)
	// Corpo acima do teto.
	expect(t, licenseCall(t, f.server, func(s *Server) http.HandlerFunc {
		return limitBody(licenseActionBodyLimit, s.revokeLicenseHandler)
	}, testAdminAccount, map[string]string{"user_id": f.alice, "track_id": f.trackID, "reason": "redistribution",
		"evidence": strings.Repeat("x", licenseActionBodyLimit)}), http.StatusBadRequest, codeInvalidRequest)
	expect(t, f.license(t, f.alice, newTestUUID(t)), http.StatusOK, "")
	if len(n.sent) != 0 {
		t.Fatalf("aviso saiu de pedido recusado: %+v", n.sent)
	}
}

// Sem e-mail configurado, nada muda: a seção 10.5 promete o aviso.
func TestNoRevocationWithoutAWayToNotify(t *testing.T) {
	f, _ := boughtByAlice(t)
	f.server.licenseNotifier = nil

	expect(t, licenseCall(t, f.server, revokeRoute, testAdminAccount,
		map[string]string{"user_id": f.alice, "track_id": f.trackID, "reason": "redistribution", "evidence": "e"}),
		http.StatusServiceUnavailable, codeInternal)
	expect(t, f.license(t, f.alice, newTestUUID(t)), http.StatusOK, "")
}

// O SMTP falhou: a revogação já está gravada, e a resposta diz que o aviso não saiu.
func TestARevocationStaysWhenTheEmailFails(t *testing.T) {
	f, n := boughtByAlice(t)
	n.fail = true

	rr := licenseCall(t, f.server, revokeRoute, testAdminAccount,
		map[string]string{"user_id": f.alice, "track_id": f.trackID, "reason": "redistribution", "evidence": "e"})
	expect(t, rr, http.StatusOK, "")
	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["email_sent"] != false {
		t.Fatalf("resposta não diz que o aviso falhou: %s", rr.Body.String())
	}
	expect(t, f.license(t, f.alice, newTestUUID(t)), http.StatusForbidden, codeEntitlementRequired)
}
