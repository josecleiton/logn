package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"slices"

	"github.com/josecleiton/logn/backend/internal/domain"
	"github.com/josecleiton/logn/backend/internal/legal"
	"github.com/josecleiton/logn/backend/internal/locale"
)

type pendingLegalDocument struct {
	Kind        string `json:"kind"`
	Version     int    `json:"version"`
	Locale      string `json:"locale"`
	EffectiveAt string `json:"effective_at"`
	Material    bool   `json:"material"`
	SHA256      string `json:"sha256"`
	// 0 quando a conta nunca aceitou; aí a data vem vazia.
	AcceptedVersion     int    `json:"accepted_version"`
	AcceptedEffectiveAt string `json:"accepted_effective_at"`
}

type pendingLegalChange struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Version int    `json:"version"`
	Change  string `json:"change"`
	Section string `json:"section"`
	Summary string `json:"summary"`
}

type pendingLegalResponse struct {
	// Alguma versão pulada é relevante: o app bloqueia até aceitar. Falso com
	// documentos na lista é o aviso único.
	Blocking  bool                   `json:"blocking"`
	Documents []pendingLegalDocument `json:"documents"`
	Changes   []pendingLegalChange   `json:"changes"`
}

// pendingLegalHandler diz o que a conta tem para aceitar e o que mudou desde o último
// aceite (ADR 0020). Quem decide se bloqueia é o servidor; o app não compara versão.
func (s *Server) pendingLegalHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.authenticate(w, r)
	if !ok {
		return
	}

	pending, err := s.repo.GetLegalPending(r.Context(), userID, locale.Negotiate(r))
	if err != nil {
		log.Printf("pendências legais não lidas: user=%s erro=%v", userID, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}

	out := pendingLegalResponse{Documents: []pendingLegalDocument{}, Changes: []pendingLegalChange{}}
	for _, d := range pending.Documents {
		doc := pendingLegalDocument{
			Kind: d.Kind, Version: d.Version, Locale: d.Locale, EffectiveAt: legal.EffectiveDate(d.EffectiveAt),
			Material: d.Material, SHA256: d.BodySHA256, AcceptedVersion: d.AcceptedVersion,
		}
		if d.AcceptedEffectiveAt != nil {
			doc.AcceptedEffectiveAt = legal.EffectiveDate(*d.AcceptedEffectiveAt)
		}
		out.Blocking = out.Blocking || d.Material
		out.Documents = append(out.Documents, doc)
	}
	for _, c := range pending.Changes {
		out.Changes = append(out.Changes, pendingLegalChange{
			ID: c.ID(), Kind: c.Kind, Version: c.Version, Change: c.Change, Section: c.Section, Summary: c.Summary,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(out)
}

type AcceptLegalDocument struct {
	Kind        string `json:"kind"`
	Version     int    `json:"version"`
	Locale      string `json:"locale"`
	SHA256      string `json:"sha256"`
	FromVersion int    `json:"from_version"`
}

type AcceptLegalRequest struct {
	Documents    []AcceptLegalDocument `json:"documents"`
	ShownChanges []string              `json:"shown_changes"`
	Client       struct {
		App      string `json:"app"`
		Platform string `json:"platform"`
	} `json:"client"`
	// "reaccept" (a tela que bloqueia) ou "notice" (a faixa do aviso único).
	Source string `json:"source"`
}

// maxShownChanges limita o que um aceite diz ter mostrado. Uma versão lista poucas
// mudanças; mais que isto é lixo.
const maxShownChanges = 64

var appVersionPattern = regexp.MustCompile(`^[0-9A-Za-z.+-]{1,32}$`)

var acceptPlatforms = []string{"ios", "android"}

// SignupClient é o `client` do cadastro, o mesmo do reaceite. Opcional: app que ainda
// não o manda grava NULL, como gravava antes.
type SignupClient struct {
	App      string `json:"app"`
	Platform string `json:"platform"`
}

// info valida o client com a mesma lista fechada do reaceite. Vazio vale (app antigo);
// valor fora da lista é pedido malformado, não NULL silencioso.
func (c SignupClient) info() (domain.ClientInfo, bool) {
	if c.App == "" && c.Platform == "" {
		return domain.ClientInfo{}, true
	}
	if !slices.Contains(acceptPlatforms, c.Platform) || (c.App != "" && !appVersionPattern.MatchString(c.App)) {
		return domain.ClientInfo{}, false
	}
	return domain.ClientInfo{App: c.App, Platform: c.Platform}, true
}

// acceptLegalHandler grava o novo aceite (ADR 0020). O pedido tem de aceitar tudo o
// que está pendente, na versão vigente e com o hash do texto que o servidor serve: é
// a prova de que o aceito é o que estava na tela. A hora é a do servidor.
func (s *Server) acceptLegalHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.authenticate(w, r)
	if !ok {
		return
	}

	var req AcceptLegalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	if req.Source != "reaccept" && req.Source != "notice" {
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	if !slices.Contains(acceptPlatforms, req.Client.Platform) ||
		(req.Client.App != "" && !appVersionPattern.MatchString(req.Client.App)) {
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	if len(req.Documents) == 0 || len(req.Documents) > len(domain.LegalKinds) || len(req.ShownChanges) > maxShownChanges {
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	// Uma língua por aceite: é a da tela que a pessoa leu.
	lang := req.Documents[0].Locale
	if !slices.Contains(legal.Locales, lang) {
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}

	pending, err := s.repo.GetLegalPending(r.Context(), userID, lang)
	if err != nil {
		log.Printf("aceite sem pendências: user=%s erro=%v", userID, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}

	// Tudo o que está pendente, e só isso. Aceitar um documento e deixar o outro
	// deixaria o app bloqueado de novo na próxima abertura.
	if len(req.Documents) != len(pending.Documents) {
		writeError(w, http.StatusConflict, codeLegalVersionOutdated)
		return
	}
	var records []domain.LegalAcceptanceRecord
	for _, want := range pending.Documents {
		i := slices.IndexFunc(req.Documents, func(d AcceptLegalDocument) bool { return d.Kind == want.Kind })
		if i < 0 {
			writeError(w, http.StatusConflict, codeLegalVersionOutdated)
			return
		}
		got := req.Documents[i]
		// Versão, língua servida, versão de origem e o texto: qualquer diferença é tela
		// velha, e o app busca de novo.
		if got.Version != want.Version || got.Locale != want.Locale || got.FromVersion != want.AcceptedVersion ||
			subtle.ConstantTimeCompare([]byte(got.SHA256), []byte(want.BodySHA256)) != 1 {
			writeError(w, http.StatusConflict, codeLegalVersionOutdated)
			return
		}
		records = append(records, domain.LegalAcceptanceRecord{
			Kind: want.Kind, Version: want.Version, Locale: want.Locale,
			BodySHA256: want.BodySHA256, FromVersion: want.AcceptedVersion,
		})
	}

	// A faixa só vale para mudança não relevante. Um cliente velho ou adulterado que
	// mande `notice` com versão relevante pendente não grava consentimento: o app busca
	// de novo e cai na tela que bloqueia.
	if req.Source == "notice" {
		for _, d := range pending.Documents {
			if d.Material {
				writeError(w, http.StatusConflict, codeLegalVersionOutdated)
				return
			}
		}
	}

	known := map[string]bool{}
	for _, c := range pending.Changes {
		known[c.ID()] = true
	}
	shown := []string{}
	for _, id := range req.ShownChanges {
		if !known[id] {
			writeError(w, http.StatusBadRequest, codeInvalidRequest)
			return
		}
		if !slices.Contains(shown, id) {
			shown = append(shown, id)
		}
	}
	// A tela que bloqueia mostra o diff inteiro. Aceite que diz ter mostrado menos não
	// prova que a pessoa viu o que mudou.
	if req.Source == "reaccept" && len(shown) != len(known) {
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}

	if err := s.repo.RecordLegalAcceptances(r.Context(), userID, records, shown, req.Client.App, req.Client.Platform, req.Source); err != nil {
		log.Printf("aceite não gravado: user=%s erro=%v", userID, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}
	w.WriteHeader(http.StatusOK)
}
