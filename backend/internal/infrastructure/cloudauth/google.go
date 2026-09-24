package cloudauth

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/api/idtoken"
)

// Validator defines the interface for verifying cloud provider tokens.
// Isolating this allows us to easily switch to AWS/Azure/etc. in the future.
type Validator interface {
	ValidateToken(ctx context.Context, authHeader string, expectedAudience string) error
}

type GoogleValidator struct{}

func NewGoogleValidator() *GoogleValidator {
	return &GoogleValidator{}
}

func (g *GoogleValidator) ValidateToken(ctx context.Context, authHeader string, expectedAudience string) error {
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return errors.New("missing or invalid authorization header")
	}
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	// Validates the OIDC token using Google's public keys.
	payload, err := idtoken.Validate(ctx, tokenString, expectedAudience)
	if err != nil {
		return err
	}

	// For Cloud Scheduler, Google is the issuer.
	if payload.Issuer != "https://accounts.google.com" {
		return errors.New("invalid issuer")
	}

	return nil
}
