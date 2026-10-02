package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/josecleiton/logn/backend/internal/domain"
	"github.com/josecleiton/logn/backend/internal/locale"
)

// tracksHandler é o catálogo: a principal e as pagas. Sem conta, responde o catálogo aberto;
// com token, marca as compradas e mantém a descontinuada de quem comprou. Token
// presente e inválido é 401, para o app renovar a sessão em vez de achar que não
// comprou nada.
//
//	@Summary	Catálogo de trilhas
//	@Tags		tracks
//	@Produce	json
//	@Security	BearerAuth
//	@Param		lang	query		string	false	"Língua do conteúdo"	Enums(pt-BR, en, es)
//	@Success	200		{array}		domain.Track
//	@Failure	401		{object}	apiError	"unauthenticated"
//	@Failure	429		{object}	apiError	"rate_limited"
//	@Failure	500		{object}	apiError	"internal"
//	@Router		/api/v1/tracks [get]
func (s *Server) tracksHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.optionalAccount(w, r)
	if !ok {
		return
	}

	lang := locale.Negotiate(r)
	tracks, err := s.repo.GetTracks(r.Context(), lang, userID)
	if err != nil {
		log.Printf("catálogo não lido: locale=%s erro=%v", lang, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}
	// Com conta, a resposta diz o que ela comprou e o que foi revogado.
	writeAccountContentJSON(w, lang, userID, tracks)
}

// trackLicenseHandler devolve a chave e o prazo offline, e registra o aparelho.
//
// Todo pedido traz `X-Device-ID` (o `identifierForVendor`). O registro não trava: não
// há limite de aparelhos até existir medida de quantos uma conta legítima usa.
//
//	@Summary	Licença de uma trilha paga
//	@Tags		tracks
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id			path		string	true	"Id da trilha (UUID)"
//	@Param		X-Device-ID	header		string	true	"Id do aparelho (UUID)"
//	@Success	200			{object}	domain.License
//	@Failure	400			{object}	apiError	"invalid_request, device_id_required"
//	@Failure	401			{object}	apiError	"unauthenticated"
//	@Failure	403			{object}	apiError	"entitlement_required"
//	@Failure	429			{object}	apiError	"rate_limited"
//	@Failure	500			{object}	apiError	"internal"
//	@Router		/api/v1/tracks/{id}/license [get]
func (s *Server) trackLicenseHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	trackID := r.PathValue("id")
	if !isUUID(trackID) {
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	deviceID := r.Header.Get("X-Device-ID")
	if !isUUID(deviceID) {
		writeError(w, http.StatusBadRequest, codeDeviceIDRequired)
		return
	}

	license, err := s.repo.IssueLicense(r.Context(), userID, trackID, deviceID, time.Now())
	if errors.Is(err, domain.ErrEntitlementRequired) {
		writeError(w, http.StatusForbidden, codeEntitlementRequired)
		return
	}
	if err != nil {
		log.Printf("licença não emitida: user=%s trilha=%s erro=%v", userID, trackID, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(license)
}

// trackPackageHandler devolve o conteúdo fechado da trilha, cifrado. Não registra
// aparelho: baixar de novo não é usar num aparelho a mais.
//
//	@Summary		Pacote cifrado de uma trilha paga
//	@Description	Bytes cifrados; a versão do conteúdo vem em `X-Content-Version`.
//	@Tags			tracks
//	@Produce		octet-stream
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Id da trilha (UUID)"
//	@Success		200	{file}		binary
//	@Failure		400	{object}	apiError	"invalid_request"
//	@Failure		401	{object}	apiError	"unauthenticated"
//	@Failure		403	{object}	apiError	"entitlement_required"
//	@Failure		429	{object}	apiError	"rate_limited"
//	@Failure		500	{object}	apiError	"internal"
//	@Router			/api/v1/tracks/{id}/package [get]
func (s *Server) trackPackageHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	trackID := r.PathValue("id")
	if !isUUID(trackID) {
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}

	blob, version, err := s.repo.BuildTrackPackage(r.Context(), userID, trackID)
	if errors.Is(err, domain.ErrEntitlementRequired) {
		writeError(w, http.StatusForbidden, codeEntitlementRequired)
		return
	}
	if err != nil {
		log.Printf("pacote não montado: user=%s trilha=%s erro=%v", userID, trackID, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Version", strconv.Itoa(version))
	w.Write(blob)
}

// isUUID aceita só a forma canônica, 8-4-4-4-12 em hexadecimal. O que vira chave de
// banco ou de registro sai de forma fechada, nunca do pedido cru.
func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !('0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F') {
				return false
			}
		}
	}
	return true
}
