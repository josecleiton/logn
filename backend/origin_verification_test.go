package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
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

// O rate limit conta por jogador, não por nó da borda — e só confia no IP do proxy
// quando o pedido provou vir dele.
func TestRateLimitKeysOnVerifiedClientIP(t *testing.T) {
	os.Setenv("ORIGIN_TRUSTED_CIDRS", "173.245.48.0/20")
	os.Setenv("ORIGIN_SHARED_SECRET", "s3cr3t")
	os.Setenv("K_SERVICE", "logn")
	defer os.Unsetenv("ORIGIN_TRUSTED_CIDRS")
	defer os.Unsetenv("ORIGIN_SHARED_SECRET")
	defer os.Unsetenv("K_SERVICE")

	origin, err := newOriginVerifierFromEnv()
	if err != nil {
		t.Fatalf("newOriginVerifierFromEnv() unexpected error: %v", err)
	}

	type hit struct {
		path     string
		secret   string
		playerIP string
		want     int
	}
	cases := []struct {
		name string
		hits []hit
	}{
		{
			name: "dois jogadores no mesmo nó da borda têm baldes separados",
			hits: []hit{
				{path: "/api/v1/auth/login", secret: "s3cr3t", playerIP: "198.51.100.7", want: http.StatusOK},
				{path: "/api/v1/auth/login", secret: "s3cr3t", playerIP: "198.51.100.8", want: http.StatusOK},
			},
		},
		{
			name: "o mesmo jogador estoura o próprio balde",
			hits: []hit{
				{path: "/api/v1/auth/login", secret: "s3cr3t", playerIP: "198.51.100.7", want: http.StatusOK},
				{path: "/api/v1/auth/login", secret: "s3cr3t", playerIP: "198.51.100.7", want: http.StatusTooManyRequests},
			},
		},
		{
			// Rota isenta aceita pedido sem segredo; trocar o header a cada pedido não
			// pode render balde novo.
			name: "sem segredo, CF-Connecting-IP forjado é ignorado",
			hits: []hit{
				{path: "/api/v1/internal/purge", playerIP: "203.0.113.1", want: http.StatusOK},
				{path: "/api/v1/internal/purge", playerIP: "203.0.113.2", want: http.StatusTooManyRequests},
			},
		},
		{
			name: "IP inválido do proxy cai no nó da borda",
			hits: []hit{
				{path: "/api/v1/auth/login", secret: "s3cr3t", playerIP: "não-é-ip", want: http.StatusOK},
				{path: "/api/v1/auth/login", secret: "s3cr3t", playerIP: "também-não", want: http.StatusTooManyRequests},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rl := newRateLimiter(1, time.Minute)
			ok := rl.wrap(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
			exempt := func(r *http.Request) bool { return r.URL.Path == "/api/v1/internal/purge" }
			handler := origin.wrap(ok, exempt)

			for i, h := range tc.hits {
				req := httptest.NewRequest(http.MethodPost, h.path, nil)
				req.RemoteAddr = "192.0.2.1:12345"
				req.Header.Set("X-Forwarded-For", "173.245.48.1")
				req.Header.Set("CF-Connecting-IP", h.playerIP)
				if h.secret != "" {
					req.Header.Set("X-Origin-Verify", h.secret)
				}
				rr := httptest.NewRecorder()
				handler.ServeHTTP(rr, req)
				if rr.Code != h.want {
					t.Fatalf("pedido %d: status %d, want %d", i, rr.Code, h.want)
				}
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
