package main

import (
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/josecleiton/logn/backend/internal/domain"
	"github.com/josecleiton/logn/backend/internal/legal"
)

type memLegalStore map[string]legal.Document

func (m memLegalStore) Latest(_ context.Context, kind legal.Kind, locale string) (legal.Document, error) {
	if d, ok := m[string(kind)+"/"+locale]; ok {
		return d, nil
	}
	return legal.Document{}, legal.ErrNotFound
}

func legalMux() http.Handler {
	doc := func(kind legal.Kind, title string) legal.Document {
		return legal.Document{
			Kind: kind, Locale: legal.PtBR, Version: 1,
			EffectiveAt: time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC),
			Body:        `<article lang="pt-BR"><header><h1>` + title + `</h1></header><section id="a"><h2>1</h2><p>Texto.</p></section></article>`,
		}
	}
	mux := http.NewServeMux()
	registerLegalRoutes(mux, memLegalStore{
		"terms/pt-BR":   doc(legal.Terms, "Termos"),
		"privacy/pt-BR": doc(legal.Privacy, "Política"),
	}, false)
	return withGzip(mux)
}

func TestLegalRoutes(t *testing.T) {
	h := legalMux()
	cases := []struct {
		method, target string
		want           int
	}{
		{http.MethodGet, "/legal/terms", http.StatusOK},
		{http.MethodGet, "/legal/privacy?embed=1", http.StatusOK},
		{http.MethodHead, "/legal/terms", http.StatusOK},
		{http.MethodGet, "/legal/cookies", http.StatusNotFound},
		{http.MethodGet, "/legal/terms/..%2f..%2fmain.go", http.StatusNotFound},
		{http.MethodPost, "/legal/terms", http.StatusMethodNotAllowed},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(c.method, c.target, nil))
		if rec.Code != c.want {
			t.Errorf("%s %s: %d, want %d", c.method, c.target, rec.Code, c.want)
		}
	}
}

// Atrás do gzip, a página sai comprimida e os cabeçalhos de língua sobrevivem.
func TestLegalRoutesBehindGzip(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/legal/terms?lang=es", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	legalMux().ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatal("página não comprimida")
	}
	if rec.Header().Get("Content-Language") != legal.PtBR {
		t.Errorf("sem espanhol no store, a página cai no português: %q", rec.Header().Get("Content-Language"))
	}
	vary := strings.Join(rec.Header().Values("Vary"), ",")
	if !strings.Contains(vary, "Accept-Language") || !strings.Contains(vary, "Accept-Encoding") {
		t.Errorf("Vary = %q", vary)
	}
	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(zr)
	if !strings.Contains(string(body), "Termos") {
		t.Error("corpo comprimido sem o documento")
	}
}

func TestCheckLegalAcceptances(t *testing.T) {
	current := map[legal.Kind]legal.Document{
		legal.Terms:   {Version: 2},
		legal.Privacy: {Version: 1},
	}
	ok := []domain.LegalAcceptance{
		{Kind: "privacy", Version: 1, Locale: "es"},
		{Kind: "terms", Version: 2, Locale: "es"},
		{Kind: "terms", Version: 2, Locale: "es"},
	}
	got, err := checkLegalAcceptances(ok, current)
	if err != nil || len(got) != 2 || got[0].Kind != "terms" || got[1].Kind != "privacy" {
		t.Fatalf("got %v, %v", got, err)
	}

	cases := map[string]struct {
		accepted []domain.LegalAcceptance
		want     error
	}{
		"vazio":           {nil, errLegalMissing},
		"só termos":       {[]domain.LegalAcceptance{{Kind: "terms", Version: 2, Locale: "en"}}, errLegalMissing},
		"língua inválida": {[]domain.LegalAcceptance{{Kind: "terms", Version: 2, Locale: "fr"}, {Kind: "privacy", Version: 1, Locale: "en"}}, errLegalMissing},
		"versão velha":    {[]domain.LegalAcceptance{{Kind: "terms", Version: 1, Locale: "en"}, {Kind: "privacy", Version: 1, Locale: "en"}}, errLegalOutdated},
		"versão futura":   {[]domain.LegalAcceptance{{Kind: "terms", Version: 3, Locale: "en"}, {Kind: "privacy", Version: 1, Locale: "en"}}, errLegalOutdated},
	}
	for name, c := range cases {
		if _, err := checkLegalAcceptances(c.accepted, current); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", name, err, c.want)
		}
	}

	if _, err := checkLegalAcceptances(ok, map[legal.Kind]legal.Document{legal.Terms: {Version: 2}}); !errors.Is(err, errLegalMissing) {
		t.Errorf("sem política publicada não há cadastro: %v", err)
	}
}
