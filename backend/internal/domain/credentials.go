package domain

import (
	"errors"
	"net/mail"
	"strings"
	"unicode/utf8"
)

var (
	ErrInvalidEmail    = errors.New("invalid email")
	ErrPasswordTooWeak = errors.New("password must have at least 8 characters")
	ErrPasswordTooLong = errors.New("password must have at most 128 characters")
)

// NormalizeEmail põe o e-mail na forma em que ele é guardado e procurado.
//
// Sem isto `A@x.com` e `a@x.com` eram duas contas, cada uma com seu OTP. Recusa o que
// não é um endereço simples: nome de exibição, quebra de linha, mais de 254 bytes.
func NormalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" || len(email) > 254 {
		return "", ErrInvalidEmail
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || addr.Name != "" {
		return "", ErrInvalidEmail
	}
	return email, nil
}

// ValidatePassword aplica o mínimo do NIST 800-63B: oito caracteres, sem regra de
// composição. O teto existe porque a senha inteira passa pelo Argon2, e sem ele cabia
// uma senha do tamanho do corpo do pedido.
func ValidatePassword(password string) error {
	n := utf8.RuneCountInString(password)
	if n < 8 {
		return ErrPasswordTooWeak
	}
	if n > 128 {
		return ErrPasswordTooLong
	}
	return nil
}
