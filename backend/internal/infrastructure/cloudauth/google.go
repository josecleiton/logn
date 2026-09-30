package cloudauth

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/api/idtoken"
)

// Validator confere o token OIDC de uma rota interna e diz quem o assinou. Quem chama
// compara o e-mail com a conta de serviço esperada: emissor e audiência certos não
// bastam, porque qualquer conta de serviço de qualquer projeto consegue um token do
// Google com a audiência que quiser, e a audiência é a URL pública do serviço.
type Validator interface {
	ValidateToken(ctx context.Context, authHeader string, expectedAudience string) (Caller, error)
}

// Caller é quem assinou o token.
type Caller struct {
	// E-mail verificado da conta de serviço. Vazio nunca volta sem erro.
	Email string
}

type GoogleValidator struct{}

func NewGoogleValidator() *GoogleValidator {
	return &GoogleValidator{}
}

func (g *GoogleValidator) ValidateToken(ctx context.Context, authHeader string, expectedAudience string) (Caller, error) {
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return Caller{}, errors.New("missing or invalid authorization header")
	}
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	// Assinatura pelas chaves públicas do Google, audiência e validade.
	payload, err := idtoken.Validate(ctx, tokenString, expectedAudience)
	if err != nil {
		return Caller{}, err
	}
	return callerFrom(payload)
}

// callerFrom confere emissor e e-mail de um token já verificado. Separado para o teste
// cobrir as recusas sem falar com o Google.
func callerFrom(payload *idtoken.Payload) (Caller, error) {
	if payload.Issuer != "https://accounts.google.com" {
		return Caller{}, errors.New("invalid issuer")
	}
	email, _ := payload.Claims["email"].(string)
	verified, _ := payload.Claims["email_verified"].(bool)
	if email == "" || !verified {
		return Caller{}, errors.New("token without a verified email")
	}
	return Caller{Email: email}, nil
}

// SameAccount compara o e-mail do token com o configurado. O Google manda o e-mail de
// conta de serviço em minúsculas; a comparação é exata contra o configurado em
// minúsculas, sem aparar nada do token. Vazio nunca bate: variável de ambiente faltando
// não pode virar "qualquer um".
func SameAccount(got, want string) bool {
	want = strings.ToLower(strings.TrimSpace(want))
	return want != "" && got == want
}
