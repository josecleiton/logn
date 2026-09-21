package domain

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestGenerateOTP(t *testing.T) {
	otp := GenerateOTP()
	if len(otp) != 6 {
		t.Errorf("Expected OTP length 6, got %v", len(otp))
	}
	
	for _, c := range otp {
		if c < '0' || c > '9' {
			t.Errorf("Expected OTP to contain only numbers, got %v", otp)
			break
		}
	}
}

// Ensure the db functions work correctly when the DB is injected.
func TestOTPRepository(t *testing.T) {
	// Simple validation test for our queries
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, "postgres://logn:lognpassword@localhost:5432/logndb?sslmode=disable")
	if err != nil {
		t.Skip("Database not available for integration test")
	}
	defer conn.Close(ctx)

	repo := NewRepository(conn)
	
	email := "test_otp@example.com"
	purpose := "test_purpose"
	
	// Clean up before test
	conn.Exec(ctx, "DELETE FROM otps WHERE email=$1 AND purpose=$2", email, purpose)

	// Test Save
	err = repo.SaveOTP(ctx, email, "123456", purpose, 5*time.Minute)
	if err != nil {
		t.Errorf("Failed to save OTP: %v", err)
	}

	// Test Verify success
	valid, err := repo.ConsumeOTP(ctx, email, "123456", purpose)
	if err != nil || !valid {
		t.Errorf("Expected valid OTP, got false or err: %v", err)
	}

	// Test Single-use (should be deleted after first verification)
	valid, _ = repo.ConsumeOTP(ctx, email, "123456", purpose)
	if valid {
		t.Errorf("Expected OTP to be invalid after first use")
	}

	// Test Expiry behavior (indirectly tested by injecting an expired time in Save)
	err = repo.SaveOTP(ctx, email, "999999", purpose, -1*time.Minute)
	if err != nil {
		t.Errorf("Failed to save expired OTP: %v", err)
	}
	
	valid, _ = repo.ConsumeOTP(ctx, email, "999999", purpose)
	if valid {
		t.Errorf("Expected expired OTP to be invalid")
	}
}
