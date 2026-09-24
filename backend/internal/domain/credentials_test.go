package domain

import (
	"strings"
	"testing"
)

func TestNormalizeEmail(t *testing.T) {
	cases := map[string]string{
		"a@x.com":       "a@x.com",
		"  A@X.Com ":    "a@x.com",
		"Jogador@Example.Com": "jogador@example.com",
	}
	for in, want := range cases {
		got, err := NormalizeEmail(in)
		if err != nil || got != want {
			t.Errorf("NormalizeEmail(%q) = %q, %v; esperava %q", in, got, err, want)
		}
	}

	for _, in := range []string{
		"",
		"sem-arroba",
		"Nome <a@x.com>",
		"a@x.com\r\nBcc: b@y.com",
		strings.Repeat("a", 250) + "@x.com",
	} {
		if _, err := NormalizeEmail(in); err == nil {
			t.Errorf("NormalizeEmail(%q) devia recusar", in)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	if ValidatePassword("") == nil || ValidatePassword("1234567") == nil {
		t.Error("senha abaixo de 8 caracteres devia ser recusada")
	}
	if err := ValidatePassword("12345678"); err != nil {
		t.Errorf("8 caracteres devia passar: %v", err)
	}
	if ValidatePassword(strings.Repeat("a", 129)) == nil {
		t.Error("senha acima de 128 caracteres devia ser recusada")
	}
}

// O hash antigo, de 64 MiB, continua verificando e é marcado para refazer. O novo não.
func TestNeedsRehash(t *testing.T) {
	legado := "$argon2id$v=19$m=65536,t=1,p=4$+WHflVRpX7CuqjkDl22cPw$63wNww35x7RbA11BqOCScXPk3AbIRru3IuzuOJ1vimA"
	if !NeedsRehash(legado) {
		t.Error("hash com parâmetros antigos devia pedir rehash")
	}

	atual, err := HashPassword("uma senha qualquer")
	if err != nil {
		t.Fatal(err)
	}
	if NeedsRehash(atual) {
		t.Errorf("hash recém-feito não devia pedir rehash: %s", atual)
	}

	if !strings.HasPrefix(DummyHash(), "$argon2id$") || NeedsRehash(DummyHash()) {
		t.Error("o hash descartável tem de acompanhar os parâmetros atuais")
	}
}
