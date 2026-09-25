package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestNewOriginVerifierFromEnv(t *testing.T) {
	tests := []struct {
		name    string
		cidrs   string
		secret  string
		wantErr bool
	}{
		{name: "Missing Both", cidrs: "", secret: "", wantErr: true},
		{name: "Missing Secret", cidrs: "10.0.0.0/8", secret: "", wantErr: true},
		{name: "Missing CIDRs", cidrs: "", secret: "s3cr3t", wantErr: true},
		{name: "Invalid CIDR", cidrs: "not-a-cidr", secret: "s3cr3t", wantErr: true},
		{name: "Valid", cidrs: "10.0.0.0/8, 173.245.48.0/20", secret: "s3cr3t", wantErr: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			os.Setenv("ORIGIN_TRUSTED_CIDRS", tc.cidrs)
			os.Setenv("ORIGIN_SHARED_SECRET", tc.secret)
			defer os.Unsetenv("ORIGIN_TRUSTED_CIDRS")
			defer os.Unsetenv("ORIGIN_SHARED_SECRET")

			_, err := newOriginVerifierFromEnv()
			if (err != nil) != tc.wantErr {
				t.Errorf("newOriginVerifierFromEnv() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestOriginVerifierWrap(t *testing.T) {
	os.Setenv("ORIGIN_TRUSTED_CIDRS", "173.245.48.0/20")
	os.Setenv("ORIGIN_SHARED_SECRET", "s3cr3t")
	os.Setenv("ORIGIN_SECRET_HEADER", "X-Origin-Verify")
	os.Setenv("K_SERVICE", "logn")
	defer os.Unsetenv("ORIGIN_TRUSTED_CIDRS")
	defer os.Unsetenv("ORIGIN_SHARED_SECRET")
	defer os.Unsetenv("ORIGIN_SECRET_HEADER")
	defer os.Unsetenv("K_SERVICE")

	origin, err := newOriginVerifierFromEnv()
	if err != nil {
		t.Fatalf("newOriginVerifierFromEnv() unexpected error: %v", err)
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	exemptInternal := func(r *http.Request) bool {
		return r.URL.Path == "/api/v1/internal/purge"
	}
	handler := origin.wrap(next, exemptInternal)

	tests := []struct {
		name           string
		path           string
		xff            string
		secretHeader   string
		expectedStatus int
	}{
		{
			name:           "Blocked (No Header, No XFF)",
			path:           "/api/v1/challenges",
			xff:            "",
			secretHeader:   "",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Blocked (Trusted IP, Wrong Secret)",
			path:           "/api/v1/challenges",
			xff:            "173.245.48.1",
			secretHeader:   "wrong",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Blocked (Untrusted IP, Right Secret)",
			path:           "/api/v1/challenges",
			xff:            "1.2.3.4",
			secretHeader:   "s3cr3t",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Allowed (Trusted IP And Right Secret)",
			path:           "/api/v1/challenges",
			xff:            "173.245.48.1",
			secretHeader:   "s3cr3t",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Allowed (Exempt Path Bypasses Check)",
			path:           "/api/v1/internal/purge",
			xff:            "",
			secretHeader:   "",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.RemoteAddr = "192.0.2.1:12345"
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}
			if tc.secretHeader != "" {
				req.Header.Set("X-Origin-Verify", tc.secretHeader)
			}
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tc.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tc.expectedStatus)
			}
		})
	}
}
