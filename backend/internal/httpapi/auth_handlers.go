package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/josecleiton/logn/backend/internal/domain"
	"github.com/josecleiton/logn/backend/internal/infrastructure/socialauth"
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
	// O e-mail da conta. No login pelo provedor o app não digitou nenhum, e é por ele
	// que a sessão sabe de quem é.
	Email string `json:"email,omitempty"`
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
func (s *Server) issueSession(ctx context.Context, w http.ResponseWriter, userID, email string, restored bool) {
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
		log.Printf("session not recorded: user=%s error=%v", userID, err)
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
		Email:            email,
	})
}

// loginHandler entra com e-mail e senha. A senha chega como foi digitada, sobre TLS, e
// é conferida com Argon2id.
//
//	@Summary		Login por e-mail e senha
//	@Description	Conta inexistente e senha errada devolvem o mesmo código. Erros seguidos travam o login por um tempo (`login_locked`, com `Retry-After`).
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			credentials	body		LoginRequest	true	"E-mail e senha"
//	@Success		200			{object}	AuthResponse
//	@Failure		400			{object}	apiError	"invalid_request"
//	@Failure		401			{object}	apiError	"invalid_credentials"
//	@Failure		429			{object}	apiError	"login_locked, rate_limited"
//	@Failure		500			{object}	apiError	"internal"
//	@Router			/api/v1/auth/login [post]
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
	noted := err == nil
	if noted {
		// Antes do Argon2 e antes de saber se a conta existe: a contagem é por e-mail
		// digitado, e o 429 sai igual para conta que existe e para a que não existe.
		// Código próprio, e não `rate_limited`: o app trava só o login por senha, e o
		// login social e a troca de senha pelo código continuam abertos.
		allowed, noteErr := s.repo.NoteLoginAttempt(ctx, email, rateKey(requestIP(r)))
		if noteErr != nil {
			log.Printf("login: attempt not counted: error=%v", noteErr)
			writeError(w, http.StatusInternalServerError, codeInternal)
			return
		}
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(int(domain.LoginAttemptWindow.Seconds())))
			writeError(w, http.StatusTooManyRequests, codeLoginLocked)
			return
		}
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

	match, compareErr := domain.ComparePasswordAndHash(ctx, req.Password, hashToCompare)
	if errors.Is(compareErr, domain.ErrArgonBusy) {
		// A senha nem foi conferida: a tentativa volta, ou o servidor ocupado trancava
		// a conta de quem só estava repetindo o pedido.
		if noted {
			// Sem o cancelamento do pedido: a espera pode ter vencido junto com ele.
			if err := s.repo.ReturnLoginAttempt(context.WithoutCancel(ctx), email, rateKey(requestIP(r))); err != nil {
				log.Printf("login: attempt not returned: error=%v", err)
			}
		}
		writeArgonBusy(w)
		return
	}
	if err != nil || user.PasswordHash == "" || compareErr != nil || !match {
		writeError(w, http.StatusUnauthorized, codeInvalidCredentials)
		return
	}

	// Senha certa: as tentativas erradas de antes não contam mais contra a conta.
	if err := s.repo.ClearLoginAttempts(ctx, email); err != nil {
		log.Printf("login: counter not reset: user=%s error=%v", user.ID, err)
	}

	// Hash de parâmetros antigos é refeito agora, enquanto a senha está na mão.
	if domain.NeedsRehash(user.PasswordHash) {
		if rehashed, err := domain.HashPassword(ctx, req.Password); err == nil {
			if err := s.repo.UpdatePasswordHash(ctx, user.ID, rehashed); err != nil {
				log.Printf("rehash not recorded: user=%s error=%v", user.ID, err)
			}
		}
	}

	// Entrar dentro da carência cancela a exclusão. Sem isto a pessoa recebia a sessão
	// e tomava 401 em todo o resto, porque a conta seguia desativada.
	restored, err := s.repo.CancelAccountDeletion(ctx, user.ID)
	if err != nil {
		log.Printf("deletion not canceled on login: user=%s error=%v", user.ID, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}
	if restored {
		log.Printf("deletion canceled by login: user=%s", user.ID)
	}

	s.issueSession(ctx, w, user.ID, user.Email, restored)
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// refreshHandler troca o refresh token por um par novo. O usado é rotacionado, e
// reusar um já rotacionado derruba todas as sessões da conta.
//
//	@Summary	Renova a sessão
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Param		token	body		RefreshRequest	true	"Refresh token"
//	@Success	200		{object}	AuthResponse
//	@Failure	400		{object}	apiError	"invalid_request"
//	@Failure	401		{object}	apiError	"session_invalid"
//	@Failure	429		{object}	apiError	"rate_limited"
//	@Failure	500		{object}	apiError	"internal"
//	@Router		/api/v1/auth/refresh [post]
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
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		// Banco fora não é sessão inválida: com 401 o app apagaria a sessão.
		log.Printf("refresh not read: error=%v", err)
		writeUnavailable(w)
		return
	}
	if err != nil || tokenRecord.ExpiresAt.Before(time.Now()) {
		writeError(w, http.StatusUnauthorized, codeSessionInvalid)
		return
	}
	// Conta desativada não renova sessão. O pedido de exclusão já revoga os tokens;
	// isto fecha a porta para o que tiver escapado, como um token emitido no meio.
	active, err := s.repo.IsUserActive(ctx, tokenRecord.UserID)
	if err != nil {
		log.Printf("refresh: account not checked: user=%s error=%v", tokenRecord.UserID, err)
		writeUnavailable(w)
		return
	}
	if !active {
		log.Printf("refresh rejected: user=%s account deactivated", tokenRecord.UserID)
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
			log.Printf("refresh rejected: user=%s token already used, sessions revoked", tokenRecord.UserID)
			writeError(w, http.StatusUnauthorized, codeSessionInvalid)
			return
		}
		// Erro aqui é do banco. O 500 deslogava como o 401; o 503 o app trata como
		// servidor ocupado e mantém a sessão.
		log.Printf("refresh not rotated: user=%s error=%v", tokenRecord.UserID, err)
		writeUnavailable(w)
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
	// O app que grava os aceites, como no reaceite.
	Client SignupClient `json:"client"`
}

// registerHandler cria a conta por e-mail, com o OTP que provou o endereço.
//
//	@Summary	Cadastro por e-mail
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Param		signup	body		RegisterRequest	true	"Conta, OTP, idade e aceites"
//	@Success	200		{object}	AuthResponse
//	@Failure	400		{object}	apiError	"invalid_request, invalid_email, password_too_short, password_too_long, invalid_country, age_not_confirmed, legal_acceptance_required"
//	@Failure	401		{object}	apiError	"otp_invalid"
//	@Failure	409		{object}	apiError	"legal_version_outdated, email_taken"
//	@Failure	429		{object}	apiError	"rate_limited"
//	@Failure	500		{object}	apiError	"internal"
//	@Router		/api/v1/auth/register [post]
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
		log.Printf("signup without legal versions: error=%v", err)
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
	client, ok := req.Client.info()
	if !ok {
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}

	valid, err := s.repo.ConsumeOTP(ctx, email, req.OTP, domain.OTPPurposeVerifyEmail)
	if err != nil || !valid {
		writeError(w, http.StatusUnauthorized, codeOTPInvalid)
		return
	}

	hashedPassword, err := domain.HashPassword(ctx, req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}

	// As boas-vindas entram na caixa de saída com a conta, e só com ela: e-mail que
	// falha não desfaz cadastro (ADR 0026).
	userID, err := s.repo.CreateUser(ctx, email, hashedPassword, req.AgeConfirmed, country, acceptances, client, locale.Negotiate(r))
	if err != nil {
		// E-mail já cadastrado, inclusive o de uma conta na carência de exclusão. A
		// mensagem não muda para esse caso: dizer "entre para recuperar" contaria a
		// qualquer um que o endereço tem conta.
		writeError(w, http.StatusConflict, codeEmailTaken)
		return
	}

	s.issueSession(ctx, w, userID, email, false)
}

type ResetPasswordRequest struct {
	Email    string `json:"email"`
	OTP      string `json:"otp"`
	Password string `json:"password"`
}

// resetPasswordHandler troca a senha com o OTP de `reset_password` e já entra.
//
//	@Summary	Redefine a senha
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Param		reset	body		ResetPasswordRequest	true	"E-mail, OTP e senha nova"
//	@Success	200		{object}	AuthResponse
//	@Failure	400		{object}	apiError	"invalid_request, invalid_email, password_too_short, password_too_long"
//	@Failure	401		{object}	apiError	"otp_invalid"
//	@Failure	429		{object}	apiError	"rate_limited"
//	@Failure	500		{object}	apiError	"internal"
//	@Router		/api/v1/auth/reset-password [post]
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

	hashedPassword, err := domain.HashPassword(ctx, req.Password)
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
		log.Printf("password not reset: error=%v", err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}
	if restored {
		log.Printf("deletion canceled by password reset: user=%s", userID)
	}
	// Quem provou pelo código que é dono do e-mail volta a entrar por senha na hora:
	// sem isto, um login travado por chute alheio seguia travado com a senha nova.
	if err := s.repo.ClearLoginAttempts(ctx, email); err != nil {
		log.Printf("password reset: login counter not reset: user=%s error=%v", userID, err)
	}

	s.issueSession(ctx, w, userID, email, restored)
}

// deleteReauthMaxAge é quanto o login no provedor pode ter para servir de prova na
// exclusão de conta.
const deleteReauthMaxAge = 5 * time.Minute

// revocationRequired são os provedores cuja autorização a exclusão tem de revogar. A
// lista é fixa, e não o que estiver ligado: desligar a Apple não pode apagar a regra.
var revocationRequired = []string{socialauth.ProviderApple}

// tokenDiscarder é o revogador que também sabe apagar o token de uma exclusão que não
// aconteceu (o GitHub). A Apple não precisa: o código dela vence em cinco minutos.
type tokenDiscarder interface {
	Discard(ctx context.Context, accessToken string) error
}

type DeleteAccountRequest struct {
	Password string `json:"password"`
	// Conta sem senha prova que é dona com um login novo no provedor (ADR 0016).
	Provider string `json:"provider,omitempty"`
	IDToken  string `json:"id_token,omitempty"`
	Nonce    string `json:"nonce,omitempty"`
	// Da Apple, junto do token: é com ele que o servidor revoga o acesso (ADR 0017).
	AuthorizationCode string `json:"authorization_code,omitempty"`
}

// deleteAccountHandler agenda a exclusão da conta do token. Quem tem senha prova com
// ela; quem entrou por provedor, com um login novo nele.
//
//	@Summary	Exclui a conta
//	@Tags		account
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		proof	body		DeleteAccountRequest	true	"Senha, ou provedor com id_token e nonce"
//	@Success	200		{object}	object{purge_after=int}	"Unix, em segundos, de quando os dados somem"
//	@Failure	400		{object}	apiError				"invalid_request"
//	@Failure	401		{object}	apiError				"unauthenticated, invalid_credentials, social_token_invalid"
//	@Failure	409		{object}	apiError				"provider_reauth_required"
//	@Failure	429		{object}	apiError				"rate_limited"
//	@Failure	500		{object}	apiError				"internal"
//	@Failure	503		{object}	apiError				"provider_disabled"
//	@Router		/api/v1/users/me/delete [post]
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

	// O access token de uma troca de exclusão do GitHub não expira sozinho. Se a
	// exclusão para antes de revogar (bilhete de outra conta, Apple exigida, erro), ele é
	// apagado aqui, em vez de ficar vivo no aparelho (ADR 0019).
	marked := false
	if d, ok := s.revokers[req.Provider].(tokenDiscarder); ok && req.AuthorizationCode != "" {
		defer func() {
			if marked {
				return
			}
			if err := d.Discard(context.WithoutCancel(ctx), req.AuthorizationCode); err != nil {
				log.Printf("token from the refused deletion not discarded: provider=%s user=%s error=%v", req.Provider, userID, err)
			}
		}()
	}

	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		// Token válido de conta que não existe mais: para o app, é sessão sem dono.
		writeError(w, http.StatusUnauthorized, codeUnauthenticated)
		return
	}

	// O `sub` que provou a posse, quando a prova foi um provedor. A revogação confere
	// que o código é dele.
	var provedSubject string
	if req.IDToken != "" {
		// A identidade do token tem de ser desta conta: um login válido de outra conta
		// no mesmo provedor não apaga nada aqui.
		id, ok := s.verifySocial(ctx, w, req.Provider, req.IDToken, req.Nonce)
		if !ok {
			return
		}
		// Login de agora, não um token que sobrou de outro: o ID token vale uma hora,
		// e excluir conta pede a pessoa na frente da tela.
		if time.Since(id.IssuedAt) > deleteReauthMaxAge {
			writeError(w, http.StatusUnauthorized, codeSocialTokenInvalid)
			return
		}
		owns, err := s.repo.HasIdentity(ctx, userID, req.Provider, id.Subject)
		if err != nil {
			log.Printf("identity not checked on deletion: user=%s error=%v", userID, err)
			writeError(w, http.StatusInternalServerError, codeInternal)
			return
		}
		if !owns {
			writeError(w, http.StatusUnauthorized, codeInvalidCredentials)
			return
		}
		// Provedor que exige revogar precisa do código para isso. Sem ele a conta
		// sairia com o acesso ainda autorizado no provedor.
		if _, needs := s.revokers[req.Provider]; needs && req.AuthorizationCode == "" {
			writeError(w, http.StatusBadRequest, codeInvalidRequest)
			return
		}
		provedSubject = id.Subject
	} else {
		match, compareErr := domain.ComparePasswordAndHash(ctx, req.Password, user.PasswordHash)
		if errors.Is(compareErr, domain.ErrArgonBusy) {
			writeArgonBusy(w)
			return
		}
		if user.PasswordHash == "" || compareErr != nil || !match {
			writeError(w, http.StatusUnauthorized, codeInvalidCredentials)
			return
		}
	}

	// Qualquer que seja a prova (senha, Google), se a conta também entra por provedor
	// que exige revogar e a prova não foi ele, a exclusão tem de passar por ele: é o
	// único jeito de ter o código que revoga o acesso.
	for _, provider := range revocationRequired {
		if provider == req.Provider && req.IDToken != "" {
			continue
		}
		linked, err := s.repo.HasProviderIdentity(ctx, userID, provider)
		if err != nil {
			log.Printf("identities not checked on deletion: user=%s error=%v", userID, err)
			writeError(w, http.StatusInternalServerError, codeInternal)
			return
		}
		if !linked {
			continue
		}
		if _, enabled := s.revokers[provider]; enabled {
			writeError(w, http.StatusConflict, codeProviderReauthRequired)
			return
		}
		// Provedor desligado depois de haver contas nele: sem como confirmar por ele,
		// segurar a exclusão deixaria a pessoa sem saída. Sai, e o log avisa que falta
		// revogar à mão.
		log.Printf("deletion with no possible revocation: provider=%s user=%s (provider disabled)", provider, userID)
	}

	purgeAfter, err := s.repo.MarkAccountForDeletion(ctx, userID)
	if err != nil {
		log.Printf("deletion not requested: user=%s error=%v", userID, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}
	// Daqui em diante o token é da revogação, que o consome.
	marked = true
	log.Printf("deletion requested: user=%s purge_after=%s", userID, purgeAfter.Format(time.RFC3339))

	// Depois de a conta estar desativada: a Apple fora do ar não segura a exclusão, que
	// é o que o jogador pediu. A falha fica no log para revogar à mão.
	// O contexto não morre com o pedido: a conta já está desativada, e o código é de uso
	// único; cair a conexão agora não pode desperdiçá-lo.
	if revoker, ok := s.revokers[req.Provider]; ok && provedSubject != "" {
		if err := revoker.Revoke(context.WithoutCancel(ctx), req.AuthorizationCode, provedSubject); err != nil {
			log.Printf("access not revoked at the provider: provider=%s user=%s error=%v", req.Provider, userID, err)
		} else {
			log.Printf("access revoked at the provider: provider=%s user=%s", req.Provider, userID)
		}
	}

	// A data em que o expurgo pode apagar a conta: o app mostra "apagada até DD/MM".
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int64{"purge_after": purgeAfter.Unix()})
}
