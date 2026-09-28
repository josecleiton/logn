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
	"github.com/josecleiton/logn/backend/internal/legal"
	"github.com/josecleiton/logn/backend/internal/locale"
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
	// A conta tinha pedido exclusão, e este login ou esta troca de senha a cancelou.
	// O app avisa uma vez. Some do JSON quando falso: app antigo ignora o campo.
	AccountRestored bool `json:"account_restored,omitempty"`
}

func hashRefreshToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// issueSession emite access e refresh para o usuário e responde com os dois.
//
// Login, registro e troca de senha repetiam os mesmos passos à mão, e o da troca de
// senha descartava os erros: sem token gravado, o app recebia uma sessão que morria no
// primeiro refresh.
func (s *Server) issueSession(ctx context.Context, w http.ResponseWriter, userID string, restored bool) {
	accessToken, err := domain.GenerateAccessToken(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}

	refreshToken, err := domain.GenerateRefreshToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}

	expiresAt := time.Now().Add(refreshTokenLifetime)
	if err := s.repo.CreateRefreshToken(ctx, userID, hashRefreshToken(refreshToken), expiresAt); err != nil {
		log.Printf("sessão não gravada: user=%s erro=%v", userID, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		UserID:           userID,
		RefreshExpiresAt: expiresAt.Unix(),
		AccountRestored:  restored,
	})
}

func (s *Server) loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}

	ctx := r.Context()

	// Senha acima do teto nem chega ao Argon2. Não entrega nada: nenhuma conta tem
	// senha desse tamanho, porque o registro recusa.
	if len(req.Password) > 4*128 {
		writeError(w, http.StatusUnauthorized, codeInvalidCredentials)
		return
	}

	var user *domain.User
	email, err := domain.NormalizeEmail(req.Email)
	if err == nil {
		user, err = s.repo.GetUserByEmail(ctx, email)
	}

	// Sem usuário, ou com conta sem senha, compara contra um hash descartável. O
	// Argon2 roda nos dois casos e o tempo de resposta não entrega quem tem conta. A
	// conta sem senha escapava disso: caía no ErrInvalidHash sem rodar o Argon2 e
	// respondia bem mais rápido.
	hashToCompare := domain.DummyHash()
	if err == nil && user.PasswordHash != "" {
		hashToCompare = user.PasswordHash
	}

	match, compareErr := domain.ComparePasswordAndHash(req.Password, hashToCompare)
	if err != nil || user.PasswordHash == "" || compareErr != nil || !match {
		writeError(w, http.StatusUnauthorized, codeInvalidCredentials)
		return
	}

	// Hash de parâmetros antigos é refeito agora, enquanto a senha está na mão.
	if domain.NeedsRehash(user.PasswordHash) {
		if rehashed, err := domain.HashPassword(req.Password); err == nil {
			if err := s.repo.UpdatePasswordHash(ctx, user.ID, rehashed); err != nil {
				log.Printf("rehash não gravado: user=%s erro=%v", user.ID, err)
			}
		}
	}

	// Entrar dentro da carência cancela a exclusão. Sem isto a pessoa recebia a sessão
	// e tomava 401 em todo o resto, porque a conta seguia desativada.
	restored, err := s.repo.CancelAccountDeletion(ctx, user.ID)
	if err != nil {
		log.Printf("exclusão não cancelada no login: user=%s erro=%v", user.ID, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}
	if restored {
		log.Printf("exclusão cancelada pelo login: user=%s", user.ID)
	}

	s.issueSession(ctx, w, user.ID, restored)
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
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}

	tokenHash := hashRefreshToken(req.RefreshToken)

	ctx := r.Context()
	tokenRecord, err := s.repo.GetRefreshToken(ctx, tokenHash)
	if err != nil || tokenRecord.ExpiresAt.Before(time.Now()) {
		writeError(w, http.StatusUnauthorized, codeSessionInvalid)
		return
	}
	// Conta desativada não renova sessão. O pedido de exclusão já revoga os tokens;
	// isto fecha a porta para o que tiver escapado, como um token emitido no meio.
	if !s.repo.IsUserActive(ctx, tokenRecord.UserID) {
		log.Printf("refresh recusado: user=%s conta desativada", tokenRecord.UserID)
		writeError(w, http.StatusUnauthorized, codeSessionInvalid)
		return
	}

	accessToken, err := domain.GenerateAccessToken(tokenRecord.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}

	// Rotação: o refresh usado morre aqui e sai outro, com prazo novo.
	//
	// Antes o refresh só renovava o access token e o prazo do refresh seguia correndo
	// desde o login — quem abria o app todo dia era deslogado no trigésimo primeiro,
	// sem ter feito nada de errado.
	newRefresh, err := domain.GenerateRefreshToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}
	expiresAt := time.Now().Add(refreshTokenLifetime)

	// Token já revogado também passa por aqui, e não é barrado antes: é a rotação que
	// reconhece o reuso e derruba as outras sessões do usuário.
	if err := s.repo.RotateRefreshToken(
		ctx, tokenHash, tokenRecord.UserID, hashRefreshToken(newRefresh), expiresAt,
	); err != nil {
		if errors.Is(err, domain.ErrRefreshTokenAlreadyUsed) {
			log.Printf("refresh recusado: user=%s token já usado, sessões revogadas", tokenRecord.UserID)
			writeError(w, http.StatusUnauthorized, codeSessionInvalid)
			return
		}
		log.Printf("refresh não rotacionado: user=%s erro=%v", tokenRecord.UserID, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
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
	Email        string `json:"email"`
	Password     string `json:"password"`
	OTP          string `json:"otp"`
	AgeConfirmed bool   `json:"age_confirmed"`
	// País considerado na confirmação de idade, ISO 3166-1 alfa-2. Vazio vale a
	// idade padrão.
	Country          string                   `json:"country"`
	LegalAcceptances []domain.LegalAcceptance `json:"legal_acceptances"`
}

func (s *Server) registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}

	email, err := domain.NormalizeEmail(req.Email)
	if err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidEmail)
		return
	}
	// A senha é conferida antes do OTP: recusá-la depois gastaria um código válido.
	if err := domain.ValidatePassword(req.Password); err != nil {
		writeError(w, http.StatusBadRequest, passwordErrorCode(err))
		return
	}

	ctx := r.Context()

	// Idade e aceite também vêm antes do OTP, pelo mesmo motivo da senha. A caixa diz
	// "tenho N anos ou mais", com o N da tabela para este país: marcá-la é a
	// declaração, e sem ela não há cadastro.
	country, ok := legal.NormalizeCountry(req.Country)
	if !ok {
		writeError(w, http.StatusBadRequest, codeInvalidCountry)
		return
	}
	if !req.AgeConfirmed {
		writeError(w, http.StatusBadRequest, codeAgeNotConfirmed)
		return
	}
	current, err := s.currentLegalVersions(ctx)
	if err != nil {
		log.Printf("cadastro sem versões legais: erro=%v", err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}
	acceptances, err := checkLegalAcceptances(req.LegalAcceptances, current)
	if errors.Is(err, errLegalOutdated) {
		writeError(w, http.StatusConflict, codeLegalVersionOutdated)
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, codeLegalAcceptanceRequired)
		return
	}

	valid, err := s.repo.ConsumeOTP(ctx, email, req.OTP, domain.OTPPurposeVerifyEmail)
	if err != nil || !valid {
		writeError(w, http.StatusUnauthorized, codeOTPInvalid)
		return
	}

	hashedPassword, err := domain.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}

	userID, err := s.repo.CreateUser(ctx, email, hashedPassword, req.AgeConfirmed, country, acceptances)
	if err != nil {
		// E-mail já cadastrado, inclusive o de uma conta na carência de exclusão. A
		// mensagem não muda para esse caso: dizer "entre para recuperar" contaria a
		// qualquer um que o endereço tem conta.
		writeError(w, http.StatusConflict, codeEmailTaken)
		return
	}

	// Boas-vindas só depois da conta criada, e fora do caminho da resposta: e-mail que
	// falha não desfaz cadastro. A língua é negociada aqui porque o pedido já terá
	// acabado quando a goroutine rodar.
	lang := locale.Negotiate(r)
	go func(email, lang string) {
		if err := s.mailer.SendWelcome(email, lang); err != nil {
			log.Printf("boas-vindas não enviadas: user=%s erro=%v", userID, err)
		}
	}(email, lang)

	s.issueSession(ctx, w, userID, false)
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
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}

	email, err := domain.NormalizeEmail(req.Email)
	if err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidEmail)
		return
	}
	if err := domain.ValidatePassword(req.Password); err != nil {
		writeError(w, http.StatusBadRequest, passwordErrorCode(err))
		return
	}

	ctx := r.Context()

	valid, err := s.repo.ConsumeOTP(ctx, email, req.OTP, domain.OTPPurposeResetPassword)
	if err != nil || !valid {
		writeError(w, http.StatusUnauthorized, codeOTPInvalid)
		return
	}

	hashedPassword, err := domain.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}

	// Troca a senha e revoga todas as sessões abertas numa transação só: quem estava
	// dentro da conta sai junto com a senha velha.
	// Cancela também uma exclusão pedida (ver ResetUserPassword).
	userID, restored, err := s.repo.ResetUserPassword(ctx, email, hashedPassword)
	if err != nil {
		// E-mail sem conta responde como código inválido. Era um 500 próprio, e
		// dava para distinguir quem tem conta.
		if errors.Is(err, domain.ErrUserNotFound) {
			writeError(w, http.StatusUnauthorized, codeOTPInvalid)
			return
		}
		log.Printf("senha não trocada: erro=%v", err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}
	if restored {
		log.Printf("exclusão cancelada pela troca de senha: user=%s", userID)
	}

	s.issueSession(ctx, w, userID, restored)
}

type DeleteAccountRequest struct {
	Password string `json:"password"`
}

func (s *Server) deleteAccountHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := s.authenticate(w, r)
	if !ok {
		return
	}

	var req DeleteAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}

	ctx := r.Context()
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		// Token válido de conta que não existe mais: para o app, é sessão sem dono.
		writeError(w, http.StatusUnauthorized, codeUnauthenticated)
		return
	}

	match, compareErr := domain.ComparePasswordAndHash(req.Password, user.PasswordHash)
	if err != nil || user.PasswordHash == "" || compareErr != nil || !match {
		writeError(w, http.StatusUnauthorized, codeInvalidCredentials)
		return
	}

	purgeAfter, err := s.repo.MarkAccountForDeletion(ctx, userID)
	if err != nil {
		log.Printf("exclusão não pedida: user=%s erro=%v", userID, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}
	log.Printf("exclusão pedida: user=%s expurgo_a_partir_de=%s", userID, purgeAfter.Format(time.RFC3339))

	// A data em que o expurgo pode apagar a conta: o app mostra "apagada até DD/MM".
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int64{"purge_after": purgeAfter.Unix()})
}
