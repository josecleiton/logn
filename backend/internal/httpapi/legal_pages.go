package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/josecleiton/logn/backend/internal/domain"
	"github.com/josecleiton/logn/backend/internal/legal"
)

// legalStore entrega ao pacote legal os documentos de `legal_documents`.
type legalStore struct {
	repo *domain.Repository
}

func (s legalStore) Latest(ctx context.Context, kind legal.Kind, locale string) (legal.Document, error) {
	doc, err := s.repo.GetLatestLegalDocument(ctx, string(kind), locale)
	if errors.Is(err, pgx.ErrNoRows) {
		return legal.Document{}, legal.ErrNotFound
	}
	if err != nil {
		return legal.Document{}, err
	}
	return legal.Document{
		Kind:        kind,
		Locale:      doc.Locale,
		Version:     doc.Version,
		EffectiveAt: doc.EffectiveAt,
		Body:        doc.BodyHTML,
	}, nil
}

// registerLegalRoutes pendura as duas páginas públicas. Separado de `main` para os
// testes montarem o mesmo mux com um store falso.
//
//	@Summary		Página pública dos termos ou da política
//	@Description	HTML pela allowlist de `internal/legal`, com CSP. Versão e vigência saem em `X-LogN-Legal-Version` e `X-LogN-Legal-Effective`. Erros em texto.
//	@Tags			legal
//	@Produce		html
//	@Param			lang		query		string	false	"Língua"	Enums(pt-BR, en, es)
//	@Param			embed		query		string	false	"1 para a versão embutida no app"
//	@Param			highlight	query		string	false	"Seções a destacar, separadas por vírgula"
//	@Success		200			{string}	string	"página"
//	@Failure		404			{string}	string	"Not found"
//	@Failure		500			{string}	string	"Internal error"
//	@Failure		503			{string}	string	"rascunho em modo estrito"
//	@Router			/legal/terms [get]
//	@Router			/legal/privacy [get]
func registerLegalRoutes(mux *http.ServeMux, store legal.Store, strict bool) {
	mux.HandleFunc("GET /legal/terms", legal.Handler(store, legal.Terms, strict))
	mux.HandleFunc("GET /legal/privacy", legal.Handler(store, legal.Privacy, strict))
}

type currentLegalVersion struct {
	Kind        string `json:"kind"`
	Version     int    `json:"version"`
	EffectiveAt string `json:"effective_at"`
}

type currentLegalResponse struct {
	Documents []currentLegalVersion `json:"documents"`
	// Idade mínima para criar conta no país de `?country=`. É o N da caixa "tenho N
	// anos ou mais" e o que o cadastro confere.
	MinAge int `json:"min_age"`
}

// currentLegalVersionsHandler diz o que o cadastro precisa aceitar e declarar: as
// versões vigentes dos documentos e a idade mínima do país (`?country=BR`). O app não
// tem outra forma de saber nenhum dos dois.
//
//	@Summary	Versões legais vigentes e idade mínima
//	@Tags		legal
//	@Produce	json
//	@Param		country	query		string	false	"País, ISO 3166-1 alfa-2"
//	@Success	200		{object}	currentLegalResponse
//	@Failure	400		{object}	apiError	"invalid_country"
//	@Failure	500		{object}	apiError	"internal"
//	@Router		/api/v1/legal/current [get]
func (s *Server) currentLegalVersionsHandler(w http.ResponseWriter, r *http.Request) {
	country, ok := legal.NormalizeCountry(r.URL.Query().Get("country"))
	if !ok {
		writeError(w, http.StatusBadRequest, codeInvalidCountry)
		return
	}
	versions, err := s.currentLegalVersions(r.Context())
	if err != nil {
		log.Printf("versões legais não lidas: erro=%v", err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}
	out := currentLegalResponse{Documents: []currentLegalVersion{}, MinAge: legal.MinimumAge(country)}
	for _, kind := range []legal.Kind{legal.Terms, legal.Privacy} {
		if doc, ok := versions[kind]; ok {
			out.Documents = append(out.Documents, currentLegalVersion{
				Kind: string(kind), Version: doc.Version, EffectiveAt: legal.EffectiveDate(doc.EffectiveAt),
			})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(out)
}

// currentLegalVersions lê a versão vigente de cada tipo. A versão é a mesma em todas
// as línguas; a portuguesa é a que prevalece e a que sempre existe.
func (s *Server) currentLegalVersions(ctx context.Context) (map[legal.Kind]legal.Document, error) {
	store := legalStore{s.repo}
	out := map[legal.Kind]legal.Document{}
	for _, kind := range []legal.Kind{legal.Terms, legal.Privacy} {
		doc, err := store.Latest(ctx, kind, legal.DefaultLocale)
		if errors.Is(err, legal.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out[kind] = doc
	}
	return out, nil
}

// Erros do aceite no cadastro. Voltam como texto curto no corpo, para o app e para o
// log distinguirem o motivo.
var (
	errLegalMissing  = errors.New("legal_acceptance_required")
	errLegalOutdated = errors.New("legal_version_outdated")
)

// checkLegalAcceptances exige o aceite da versão vigente dos dois documentos, numa
// língua servida, e devolve só esses dois — um por tipo, sem repetição.
//
// Antes o cadastro gravava o que viesse, inclusive nada: lista vazia criava conta sem
// aceite nenhum, e qualquer número de versão passava.
func checkLegalAcceptances(accepted []domain.LegalAcceptance, current map[legal.Kind]legal.Document) ([]domain.LegalAcceptance, error) {
	var out []domain.LegalAcceptance
	for _, kind := range []legal.Kind{legal.Terms, legal.Privacy} {
		doc, ok := current[kind]
		if !ok {
			// Sem documento publicado não há o que aceitar, e não há cadastro.
			return nil, errLegalMissing
		}
		i := slices.IndexFunc(accepted, func(a domain.LegalAcceptance) bool { return a.Kind == string(kind) })
		if i < 0 || !slices.Contains(legal.Locales, accepted[i].Locale) {
			return nil, errLegalMissing
		}
		if accepted[i].Version != doc.Version {
			return nil, errLegalOutdated
		}
		out = append(out, accepted[i])
	}
	return out, nil
}
