package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/josecleiton/logn/backend/internal/domain"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// refreshTokenLifetime é o prazo de cada refresh token. Com rotação ele reinicia a
// cada uso, então quem abre o app dentro do prazo nunca é deslogado.
const refreshTokenLifetime = 30 * 24 * time.Hour

type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	// Quem é o dono da sessão. Sem este campo o cliente não tinha como saber, e o
	// sync subia com o literal "user_1", que o Postgres recusa como UUID.
	UserID string `json:"user_id"`
	// Até quando a sessão vale, em segundos desde a época. É o que deixa o app seguir
	// funcionando sem rede: sem saber o prazo, ele só podia perguntar ao servidor.
	RefreshExpiresAt int64 `json:"refresh_expires_at"`
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

	expiresAt := time.Now().Add(refreshTokenLifetime)
	err = s.repo.CreateRefreshToken(ctx, user.ID, tokenHash, expiresAt)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		UserID:           user.ID,
		RefreshExpiresAt: expiresAt.Unix(),
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

	// Rotação: o refresh usado morre aqui e sai outro, com prazo novo.
	//
	// Antes o refresh só renovava o access token e o prazo do refresh seguia correndo
	// desde o login — quem abria o app todo dia era deslogado no trigésimo primeiro,
	// sem ter feito nada de errado.
	newRefresh, err := domain.GenerateRefreshToken()
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	newHash := sha256.Sum256([]byte(newRefresh))
	expiresAt := time.Now().Add(refreshTokenLifetime)

	if err := s.repo.RotateRefreshToken(
		ctx, tokenHash, tokenRecord.UserID, hex.EncodeToString(newHash[:]), expiresAt,
	); err != nil {
		if errors.Is(err, domain.ErrRefreshTokenAlreadyUsed) {
			// Alguém apresentou um token já trocado. Pode ser corrida do próprio app,
			// pode ser cópia — de qualquer forma não se emite sessão a partir dele.
			log.Printf("refresh recusado: user=%s token já usado", tokenRecord.UserID)
			http.Error(w, "Invalid or expired refresh token", http.StatusUnauthorized)
			return
		}
		log.Printf("refresh não rotacionado: user=%s erro=%v", tokenRecord.UserID, err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		AccessToken:      accessToken,
		RefreshToken:     newRefresh,
		UserID:           tokenRecord.UserID,
		RefreshExpiresAt: expiresAt.Unix(),
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

	expiresAt := time.Now().Add(refreshTokenLifetime)
	err = s.repo.CreateRefreshToken(ctx, userID, tokenHash, expiresAt)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		UserID:           userID,
		RefreshExpiresAt: expiresAt.Unix(),
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
	expiresAt := time.Now().Add(refreshTokenLifetime)
	_ = s.repo.CreateRefreshToken(ctx, user.ID, tokenHash, expiresAt)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		RefreshExpiresAt: expiresAt.Unix(),
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		UserID:           user.ID,
	})
}
