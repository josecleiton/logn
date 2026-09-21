package domain

import (
	"strings"
	"testing"
)

func TestHashPasswordAndCompare(t *testing.T) {
	password := "super_secure_password"
	
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Errorf("Expected hash to start with $argon2id$, got %s", hash)
	}

	match, err := ComparePasswordAndHash(password, hash)
	if err != nil {
		t.Fatalf("Failed to compare hash: %v", err)
	}
	if !match {
		t.Error("Expected password to match hash")
	}

	matchFalse, err := ComparePasswordAndHash("wrong_password", hash)
	if err != nil {
		t.Fatalf("Failed to compare wrong password: %v", err)
	}
	if matchFalse {
		t.Error("Expected wrong password to not match hash")
	}
}

func TestJWTGeneration(t *testing.T) {
	userID := "123456-uuid-mock"
	token, err := GenerateAccessToken(userID)
	if err != nil {
		t.Fatalf("Failed to generate access token: %v", err)
	}
	
	if len(token) == 0 {
		t.Error("Generated access token is empty")
	}
	
	refresh, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("Failed to generate refresh token: %v", err)
	}
	
	if len(refresh) == 0 {
		t.Error("Generated refresh token is empty")
	}
}
