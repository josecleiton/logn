package main

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
