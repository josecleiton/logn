package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/josecleiton/logn/backend/internal/domain"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func (s *Server) loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	
	// Dummy hash to prevent user enumeration via timing attacks
	// This hash was generated with the same argon2 parameters used by the app.
	dummyHash := "$argon2id$v=19$m=65536,t=1,p=4$+WHflVRpX7CuqjkDl22cPw$63wNww35x7RbA11BqOCScXPk3AbIRru3IuzuOJ1vimA"
	
	var hashToCompare string
	if err != nil {
		hashToCompare = dummyHash
	} else {
		hashToCompare = user.PasswordHash
	}

	match, compareErr := domain.ComparePasswordAndHash(req.Password, hashToCompare)
	if err != nil || compareErr != nil || !match {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	accessToken, err := domain.GenerateAccessToken(user.ID)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	refreshToken, err := domain.GenerateRefreshToken()
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Hash the refresh token before storing it
	hash := sha256.Sum256([]byte(refreshToken))
	tokenHash := hex.EncodeToString(hash[:])
	
	err = s.repo.CreateRefreshToken(ctx, user.ID, tokenHash, time.Now().Add(30*24*time.Hour))
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (s *Server) refreshHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	hash := sha256.Sum256([]byte(req.RefreshToken))
	tokenHash := hex.EncodeToString(hash[:])

	ctx := context.Background()
	tokenRecord, err := s.repo.GetRefreshToken(ctx, tokenHash)
	if err != nil || tokenRecord.Revoked || tokenRecord.ExpiresAt.Before(time.Now()) {
		http.Error(w, "Invalid or expired refresh token", http.StatusUnauthorized)
		return
	}

	accessToken, err := domain.GenerateAccessToken(tokenRecord.UserID)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"access_token": accessToken,
	})
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	OTP      string `json:"otp"`
}

func (s *Server) registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	// 1. Verify OTP
	valid, err := s.repo.ConsumeOTP(ctx, req.Email, req.OTP, "verify_email")
	if err != nil || !valid {
		http.Error(w, "Invalid or expired OTP", http.StatusUnauthorized)
		return
	}

	// 2. Hash Password
	hashedPassword, err := domain.HashPassword(req.Password)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// 3. Create User
	userID, err := s.repo.CreateUser(ctx, req.Email, hashedPassword)
	if err != nil {
		// Usually indicates email already exists
		http.Error(w, "Error creating user: email might already be registered", http.StatusConflict)
		return
	}

	// 4. Generate Tokens
	accessToken, err := domain.GenerateAccessToken(userID)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	refreshToken, err := domain.GenerateRefreshToken()
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	hash := sha256.Sum256([]byte(refreshToken))
	tokenHash := hex.EncodeToString(hash[:])
	
	err = s.repo.CreateRefreshToken(ctx, userID, tokenHash, time.Now().Add(30*24*time.Hour))
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

type ResetPasswordRequest struct {
	Email    string `json:"email"`
	OTP      string `json:"otp"`
	Password string `json:"password"`
}

func (s *Server) resetPasswordHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	// 1. Consume OTP
	valid, err := s.repo.ConsumeOTP(ctx, req.Email, req.OTP, "reset_password")
	if err != nil || !valid {
		http.Error(w, "Invalid or expired OTP", http.StatusUnauthorized)
		return
	}

	// 2. Hash New Password
	hashedPassword, err := domain.HashPassword(req.Password)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// 3. Update User Password
	err = s.repo.UpdateUserPassword(ctx, req.Email, hashedPassword)
	if err != nil {
		http.Error(w, "Error updating password", http.StatusInternalServerError)
		return
	}

	// 4. (Optional) Auto-login the user after reset
	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		http.Error(w, "Internal error fetching user", http.StatusInternalServerError)
		return
	}

	accessToken, _ := domain.GenerateAccessToken(user.ID)
	refreshToken, _ := domain.GenerateRefreshToken()
	
	hash := sha256.Sum256([]byte(refreshToken))
	tokenHash := hex.EncodeToString(hash[:])
	_ = s.repo.CreateRefreshToken(ctx, user.ID, tokenHash, time.Now().Add(30*24*time.Hour))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}
