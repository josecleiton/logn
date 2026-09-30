package cloudauth

import (
	"context"
	"testing"

	"google.golang.org/api/idtoken"
)

const account = "logn-admin@example-project.iam.gserviceaccount.com"

func TestCallerNeedsGoogleAndAVerifiedEmail(t *testing.T) {
	for name, p := range map[string]*idtoken.Payload{
		"outro emissor":          {Issuer: "https://evil.example.com", Claims: map[string]any{"email": account, "email_verified": true}},
		"emissor sem https":      {Issuer: "accounts.google.com", Claims: map[string]any{"email": account, "email_verified": true}},
		"sem e-mail":             {Issuer: "https://accounts.google.com", Claims: map[string]any{"email_verified": true}},
		"e-mail não verificado":  {Issuer: "https://accounts.google.com", Claims: map[string]any{"email": account, "email_verified": false}},
		"verificado como texto":  {Issuer: "https://accounts.google.com", Claims: map[string]any{"email": account, "email_verified": "true"}},
		"e-mail que não é texto": {Issuer: "https://accounts.google.com", Claims: map[string]any{"email": 42, "email_verified": true}},
	} {
		if _, err := callerFrom(p); err == nil {
			t.Errorf("%s: aceitou", name)
		}
	}

	c, err := callerFrom(&idtoken.Payload{Issuer: "https://accounts.google.com", Claims: map[string]any{"email": account, "email_verified": true}})
	if err != nil || c.Email != account {
		t.Fatalf("token válido: %+v %v", c, err)
	}
}

func TestSameAccountIsExact(t *testing.T) {
	cases := []struct {
		got, want string
		ok        bool
	}{
		{account, account, true},
		{account, "  " + account + "\n", true},
		{account, "LOGN-ADMIN@example-project.iam.gserviceaccount.com", true},
		{"", "", false},
		{account, "", false},
		{"", account, false},
		{account + ".evil", account, false},
		{" " + account, account, false},
		{"LOGN-ADMIN@example-project.iam.gserviceaccount.com", account, false},
		{"other@example-project.iam.gserviceaccount.com", account, false},
	}
	for _, c := range cases {
		if got := SameAccount(c.got, c.want); got != c.ok {
			t.Errorf("SameAccount(%q, %q) = %v", c.got, c.want, got)
		}
	}
}

func TestAMissingBearerIsRefused(t *testing.T) {
	for _, h := range []string{"", "Basic abc", "bearer abc", "Token abc"} {
		if _, err := NewGoogleValidator().ValidateToken(context.Background(), h, "https://example.run.app"); err == nil {
			t.Errorf("%q: aceitou", h)
		}
	}
}
