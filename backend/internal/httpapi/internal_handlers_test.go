package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/josecleiton/logn/backend/internal/domain"
)

type mockCloudValidator struct {
	shouldFail bool
}

func (m *mockCloudValidator) ValidateToken(ctx context.Context, authHeader string, expectedAudience string) error {
	if m.shouldFail || authHeader == "" {
		return errors.New("invalid token")
	}
	return nil
}

func TestPurgeHandler(t *testing.T) {
	pool := setupTestDB(t)
	repo := domain.NewRepository(pool)

	tests := []struct {
		name           string
		method         string
		audience       string
		authHeader     string
		shouldFail     bool
		expectedStatus int
	}{
		{
			name:           "Method Not Allowed",
			method:         http.MethodGet,
			audience:       "https://my-app",
			authHeader:     "Bearer valid_token",
			shouldFail:     false,
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "Endpoint Disabled (No Audience Config)",
			method:         http.MethodPost,
			audience:       "",
			authHeader:     "Bearer valid_token",
			shouldFail:     false,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Unauthorized (No Header)",
			method:         http.MethodPost,
			audience:       "https://my-app",
			authHeader:     "",
			shouldFail:     false,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Unauthorized (Invalid Token)",
			method:         http.MethodPost,
			audience:       "https://my-app",
			authHeader:     "Bearer invalid_token",
			shouldFail:     true,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Authorized",
			method:         http.MethodPost,
			audience:       "https://my-app",
			authHeader:     "Bearer valid_token",
			shouldFail:     false,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			os.Setenv("CLOUD_SCHEDULER_AUDIENCE", tc.audience)
			defer os.Unsetenv("CLOUD_SCHEDULER_AUDIENCE")

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
