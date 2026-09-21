package domain

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"
)

func GenerateOTP() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	return fmt.Sprintf("%06d", n.Int64())
}

func (r *Repository) SaveOTP(ctx context.Context, email, code, purpose string, duration time.Duration) error {
	expiresAt := time.Now().Add(duration)
	query := `
		INSERT INTO otps (email, otp_code, purpose, expires_at) 
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (email, purpose) 
		DO UPDATE SET otp_code = EXCLUDED.otp_code, expires_at = EXCLUDED.expires_at
	`
	_, err := r.db.Exec(ctx, query, email, code, purpose, expiresAt)
	return err
}

func (r *Repository) VerifyOTP(ctx context.Context, email, code, purpose string) (bool, error) {
	query := `
		SELECT otp_code, expires_at FROM otps 
		WHERE email = $1 AND purpose = $2
	`
	var dbCode string
	var expiresAt time.Time
	
	err := r.db.QueryRow(ctx, query, email, purpose).Scan(&dbCode, &expiresAt)
	if err != nil {
		return false, err // Usually pgx.ErrNoRows
	}

	if dbCode != code || time.Now().After(expiresAt) {
		return false, nil
	}

	// Delete after successful use
	_, _ = r.db.Exec(ctx, `DELETE FROM otps WHERE email = $1 AND purpose = $2`, email, purpose)
	
	return true, nil
}
