package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/josecleiton/logn/backend/internal/domain"
)

// decodeLeaderboardBody lê o corpo das rotas do placar: campo desconhecido é 400, e
// corpo acima do teto é 413 `body_too_large`, não o 400 que o JSON cortado daria.
func decodeLeaderboardBody(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(v)
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeError(w, http.StatusRequestEntityTooLarge, codeBodyTooLarge)
		return false
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return false
	}
	return true
}

// leaderboardActionRequest é o corpo da rota interna: a ação, o motivo e exatamente um
// jeito de achar a conta.
type leaderboardActionRequest struct {
	UserID     string `json:"user_id"`
	Nickname   string `json:"nickname"`
	AnonNumber int    `json:"anon_number"`
	Email      string `json:"email"`
	Action     string `json:"action"`
	Reason     string `json:"reason"`
}

// leaderboardActionHandler modera o placar e atende a objeção (seção 8 do PRD). Quem
// chama é uma pessoa pelo `just leaderboard-*`, com a conta de administração (ADR 0021).
//
//	@Summary		Moderação do placar
//	@Description	Só com ID token OIDC do Google, da conta `ADMIN_SERVICE_ACCOUNT`. A conta sai de exatamente um de user_id, nickname, anon_number ou email.
//	@Tags			internal
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			action	body		leaderboardActionRequest	true	"Conta, ação e motivo"
//	@Success		200		{object}	object{status=string,action=string}
//	@Failure		400		{object}	apiError	"invalid_request"
//	@Failure		401		{string}	string		"Unauthorized"
//	@Failure		403		{string}	string		"Forbidden"
//	@Failure		404		{object}	apiError	"user_not_found"
//	@Failure		409		{object}	apiError	"leaderboard_no_change"
//	@Failure		413		{object}	apiError	"body_too_large"
//	@Failure		429		{object}	apiError	"rate_limited"
//	@Failure		500		{object}	apiError	"internal"
//	@Router			/api/v1/internal/leaderboard/actions [post]
func (s *Server) leaderboardActionHandler(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.internalCaller(w, r, envAdminAccount)
	if !ok {
		return
	}
	var req leaderboardActionRequest
	if !decodeLeaderboardBody(w, r, &req) {
		return
	}
	if req.UserID != "" && !isUUID(req.UserID) {
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}

	userID, err := s.repo.ApplyLeaderboardAction(r.Context(), domain.LeaderboardAction{
		Target: domain.LeaderboardTarget{
			UserID: req.UserID, Nickname: req.Nickname, AnonNumber: req.AnonNumber, Email: req.Email,
		},
		Action: req.Action, Reason: req.Reason, Actor: actor,
	})
	switch {
	case errors.Is(err, domain.ErrInvalidLeaderboardAction):
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	case errors.Is(err, domain.ErrUserNotFound):
		writeError(w, http.StatusNotFound, codeUserNotFound)
		return
	case errors.Is(err, domain.ErrLeaderboardNoChange):
		writeError(w, http.StatusConflict, codeLeaderboardNoChange)
		return
	case err != nil:
		log.Printf("leaderboard action not recorded: action=%q error=%v", clipForLog(req.Action), err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}
	// O e-mail e o motivo não vão para o log; o motivo fica em leaderboard_actions.
	log.Printf("leaderboard moderated: user=%s action=%s by=%s", userID, req.Action, actor)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "action": req.Action})
}

type nicknameRequest struct {
	Nickname string `json:"nickname"`
}

type nicknameResponse struct {
	Nickname string `json:"nickname"`
}

// nicknameHandler grava o apelido da conta do token (seção 5.3 do PRD). É uma escolha
// só: uma segunda tentativa ouve `nickname_locked`.
//
//	@Summary	Escolher o apelido do placar
//	@Tags		account
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		body	body		nicknameRequest	true	"Apelido"
//	@Success	200		{object}	nicknameResponse
//	@Failure	400		{object}	apiError	"invalid_request, nickname_invalid"
//	@Failure	401		{object}	apiError	"unauthenticated"
//	@Failure	409		{object}	apiError	"nickname_reserved, nickname_taken, nickname_locked"
//	@Failure	413		{object}	apiError	"body_too_large"
//	@Failure	429		{object}	apiError	"rate_limited"
//	@Failure	500		{object}	apiError	"internal"
//	@Router		/api/v1/profile/nickname [put]
func (s *Server) nicknameHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	if s.nicknameUserLimiter != nil && !s.nicknameUserLimiter.allow(userID) {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, codeRateLimited)
		return
	}
	var req nicknameRequest
	if !decodeLeaderboardBody(w, r, &req) {
		return
	}

	nickname, err := s.repo.SetNickname(r.Context(), userID, req.Nickname)
	switch {
	case errors.Is(err, domain.ErrNicknameInvalid):
		writeError(w, http.StatusBadRequest, codeNicknameInvalid)
	case errors.Is(err, domain.ErrNicknameReserved):
		writeError(w, http.StatusConflict, codeNicknameReserved)
	case errors.Is(err, domain.ErrNicknameTaken):
		writeError(w, http.StatusConflict, codeNicknameTaken)
	case errors.Is(err, domain.ErrNicknameLocked):
		writeError(w, http.StatusConflict, codeNicknameLocked)
	case err != nil:
		log.Printf("nickname not recorded: user=%s error=%v", userID, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
	default:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(nicknameResponse{Nickname: nickname})
	}
}

// leaderboardHandler devolve o placar geral de XP para a conta do token
// (docs/specs/logn_placar_spec.md, seção 5.2). O `user_id` sai só do token; a resposta
// não traz id nem e-mail de ninguém.
//
//	@Summary	Placar geral de XP
//	@Tags		account
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	domain.Leaderboard
//	@Failure	401	{object}	apiError	"unauthenticated"
//	@Failure	429	{object}	apiError	"rate_limited"
//	@Failure	500	{object}	apiError	"internal"
//	@Router		/api/v1/leaderboard [get]
func (s *Server) leaderboardHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	board, err := s.repo.GetLeaderboard(r.Context(), userID)
	if errors.Is(err, domain.ErrUserNotFound) {
		writeError(w, http.StatusUnauthorized, codeUnauthenticated)
		return
	}
	if err != nil {
		log.Printf("leaderboard not read: user=%s error=%v", userID, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}
	// A linha do jogador é dele: nada de cache compartilhado no caminho.
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(board)
}
