package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/josecleiton/logn/backend/internal/domain"
	"github.com/josecleiton/logn/backend/internal/infrastructure/socialauth"
)

// legalFixture publica versões de mentira dos dois documentos acima da vigente, com as
// mudanças de cada uma, e apaga tudo no fim. Os textos são de fixture: nenhum é
// documento de verdade.
type legalFixture struct {
	*socialFixture
	base int // a vigente de antes do teste
	user string
}

func newLegalFixture(t *testing.T) *legalFixture {
	t.Helper()
	sf := newSocialFixture(t, fakeVerifier{})
	f := &legalFixture{socialFixture: sf, base: sf.terms}
	if sf.privacy > f.base {
		f.base = sf.privacy
	}
	// A conta aceitou a vigente de antes, como no cadastro.
	f.user = sf.seedPasswordUser(t, sf.email("aceite"))
	for _, kind := range domain.LegalKinds {
		version := sf.terms
		if kind == "privacy" {
			version = sf.privacy
		}
		if _, err := sf.pool.Exec(context.Background(), `
			INSERT INTO legal_acceptances (user_id, kind, version, locale) VALUES ($1, $2, $3, 'pt-BR')
		`, f.user, kind, version); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

// publish põe no ar a versão `base+offset` dos dois documentos, nas três línguas.
func (f *legalFixture) publish(t *testing.T, offset int, material bool, effective time.Time) int {
	t.Helper()
	v := f.base + offset
	ctx := context.Background()
	for _, kind := range domain.LegalKinds {
		for _, loc := range []string{"pt-BR", "en", "es"} {
			body := fmt.Sprintf(`<article class="legal-doc"><section id="fixture"><h2>Fixture %s v%d %s</h2></section></article>`, kind, v, loc)
			if _, err := f.pool.Exec(ctx, `
				INSERT INTO legal_documents (kind, locale, version, effective_at, material, body_html)
				VALUES ($1, $2, $3, $4, $5, $6)
			`, kind, loc, v, effective, material, body); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Cleanup(func() {
		f.pool.Exec(ctx, `DELETE FROM legal_document_changes WHERE version = $1`, v)
		f.pool.Exec(ctx, `DELETE FROM legal_acceptances WHERE version = $1`, v)
		f.pool.Exec(ctx, `DELETE FROM legal_documents WHERE version = $1`, v)
	})
	return v
}

func (f *legalFixture) change(t *testing.T, kind string, version int, loc, change, summary string) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(), `
		INSERT INTO legal_document_changes (kind, version, locale, position, change, section_id, summary)
		VALUES ($1::varchar, $2::int, $3::varchar,
			(SELECT COUNT(*) FROM legal_document_changes WHERE kind = $1::varchar AND version = $2::int AND locale = $3::varchar),
			$4, 'fixture', $5)
	`, kind, version, loc, change, summary); err != nil {
		t.Fatal(err)
	}
}

func (f *legalFixture) pending(t *testing.T, lang string) pendingLegalResponse {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/legal/pending", nil)
	req.Header.Set("Authorization", bearer(t, f.user))
	req.Header.Set("Accept-Language", lang)
	rec := httptest.NewRecorder()
	f.s.pendingLegalHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("pending: %d %s", rec.Code, rec.Body.String())
	}
	var out pendingLegalResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func (f *legalFixture) accept(t *testing.T, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/legal/accept", strings.NewReader(string(raw)))
	req.Header.Set("Authorization", bearer(t, f.user))
	rec := httptest.NewRecorder()
	f.s.acceptLegalHandler(rec, req)
	return rec
}

// acceptBody monta o aceite do que o pending devolveu, como o app faz.
func acceptBody(p pendingLegalResponse, source string) map[string]any {
	var docs []map[string]any
	for _, d := range p.Documents {
		docs = append(docs, map[string]any{
			"kind": d.Kind, "version": d.Version, "locale": d.Locale, "sha256": d.SHA256, "from_version": d.AcceptedVersion,
		})
	}
	shown := []string{}
	for _, c := range p.Changes {
		shown = append(shown, c.ID)
	}
	return map[string]any{
		"documents": docs, "shown_changes": shown, "source": source,
		"client": map[string]string{"app": "1.8.0", "platform": "ios"},
	}
}

func TestLegalPendingIsEmptyWhenCurrentIsAccepted(t *testing.T) {
	f := newLegalFixture(t)
	p := f.pending(t, "pt-BR")
	if p.Blocking || len(p.Documents) != 0 || len(p.Changes) != 0 {
		t.Fatalf("nada pendente, veio %+v", p)
	}
}

func TestLegalPendingSumsSkippedVersionsAndBlocksOnAnyMaterial(t *testing.T) {
	f := newLegalFixture(t)
	past := time.Now().Add(-time.Hour)
	mid := f.publish(t, 1, true, past) // relevante no meio
	top := f.publish(t, 2, false, past)
	// A do meio só tem português; a de cima tem inglês.
	f.change(t, "terms", mid, "pt-BR", "added", "mudança do meio")
	f.change(t, "terms", top, "pt-BR", "changed", "mudança de cima")
	f.change(t, "terms", top, "en", "changed", "top change")

	p := f.pending(t, "en")
	if !p.Blocking {
		t.Fatal("uma relevante pulada no meio bloqueia, mesmo com a vigente não relevante")
	}
	if len(p.Documents) != 2 {
		t.Fatalf("os dois documentos pendentes, veio %+v", p.Documents)
	}
	for _, d := range p.Documents {
		if d.Version != top || d.Locale != "en" || len(d.SHA256) != 64 || d.AcceptedVersion == 0 || d.AcceptedEffectiveAt == "" {
			t.Fatalf("documento pendente errado: %+v", d)
		}
	}
	if len(p.Changes) != 2 || p.Changes[0].Summary != "mudança do meio" || p.Changes[1].Summary != "top change" {
		t.Fatalf("mudanças somadas, na língua pedida e com o português de reserva: %+v", p.Changes)
	}
	if p.Changes[0].ID != fmt.Sprintf("terms:%d:fixture", mid) {
		t.Fatalf("id da mudança: %s", p.Changes[0].ID)
	}
}

// Versão publicada antes da vigência não está no ar.
func TestLegalFutureVersionIsNotCurrent(t *testing.T) {
	f := newLegalFixture(t)
	f.publish(t, 1, true, time.Now().Add(24*time.Hour))
	if p := f.pending(t, "pt-BR"); len(p.Documents) != 0 {
		t.Fatalf("versão futura já cobrada: %+v", p.Documents)
	}
	current, err := f.s.currentLegalVersions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if current["terms"].Version > f.base {
		t.Fatalf("versão futura virou a vigente do cadastro: %d", current["terms"].Version)
	}
}

func TestLegalAcceptRecordsTheProof(t *testing.T) {
	f := newLegalFixture(t)
	v := f.publish(t, 1, true, time.Now().Add(-time.Hour))
	f.change(t, "terms", v, "pt-BR", "added", "mudança")
	p := f.pending(t, "pt-BR")

	if rec := f.accept(t, acceptBody(p, "reaccept")); rec.Code != http.StatusOK {
		t.Fatalf("aceite: %d %s", rec.Code, rec.Body.String())
	}
	if after := f.pending(t, "pt-BR"); len(after.Documents) != 0 {
		t.Fatalf("aceite não resolveu a pendência: %+v", after)
	}

	var sha, source, platform, app string
	var from int
	var shown []string
	err := f.pool.QueryRow(context.Background(), `
		SELECT body_sha256, source, platform, app_version, from_version, shown_changes
		FROM legal_acceptances WHERE user_id = $1 AND kind = 'terms' AND version = $2
	`, f.user, v).Scan(&sha, &source, &platform, &app, &from, &shown)
	if err != nil {
		t.Fatal(err)
	}
	if sha != p.Documents[0].SHA256 || source != "reaccept" || platform != "ios" || app != "1.8.0" ||
		from != p.Documents[0].AcceptedVersion || len(shown) != 1 {
		t.Fatalf("registro do aceite incompleto: sha=%s source=%s platform=%s app=%s from=%d shown=%v", sha, source, platform, app, from, shown)
	}
}

func TestLegalAcceptRefuses(t *testing.T) {
	f := newLegalFixture(t)
	v := f.publish(t, 1, true, time.Now().Add(-time.Hour))
	f.change(t, "terms", v, "pt-BR", "added", "mudança")
	p := f.pending(t, "pt-BR")

	cases := []struct {
		name   string
		mut    func(map[string]any)
		status int
		code   string
	}{
		{"hash de outro texto", func(b map[string]any) {
			b["documents"].([]map[string]any)[0]["sha256"] = strings.Repeat("0", 64)
		}, 409, codeLegalVersionOutdated},
		{"só um dos documentos", func(b map[string]any) {
			b["documents"] = b["documents"].([]map[string]any)[:1]
		}, 409, codeLegalVersionOutdated},
		{"versão velha", func(b map[string]any) {
			b["documents"].([]map[string]any)[0]["version"] = f.base
		}, 409, codeLegalVersionOutdated},
		{"origem errada", func(b map[string]any) {
			b["documents"].([]map[string]any)[0]["from_version"] = 0
		}, 409, codeLegalVersionOutdated},
		{"língua diferente da servida", func(b map[string]any) {
			for _, d := range b["documents"].([]map[string]any) {
				d["locale"] = "en"
			}
		}, 409, codeLegalVersionOutdated},
		{"mudança que não existe", func(b map[string]any) {
			b["shown_changes"] = []string{"terms:1:inventada"}
		}, 400, codeInvalidRequest},
		{"fonte desconhecida", func(b map[string]any) { b["source"] = "signup" }, 400, codeInvalidRequest},
		{"plataforma desconhecida", func(b map[string]any) {
			b["client"] = map[string]string{"app": "1.8.0", "platform": "symbian"}
		}, 400, codeInvalidRequest},
		{"versão do app com lixo", func(b map[string]any) {
			b["client"] = map[string]string{"app": "<script>", "platform": "ios"}
		}, 400, codeInvalidRequest},
		{"sem documentos", func(b map[string]any) { b["documents"] = []map[string]any{} }, 400, codeInvalidRequest},
		// A faixa não serve de consentimento para versão relevante.
		{"faixa com versão relevante", func(b map[string]any) {
			b["source"] = "notice"
			b["shown_changes"] = []string{}
		}, 409, codeLegalVersionOutdated},
		{"tela que mostrou menos que o diff", func(b map[string]any) { b["shown_changes"] = []string{} }, 400, codeInvalidRequest},
		{"mudanças demais", func(b map[string]any) {
			many := make([]string, maxShownChanges+1)
			for i := range many {
				many[i] = fmt.Sprintf("terms:1:s%d", i)
			}
			b["shown_changes"] = many
		}, 400, codeInvalidRequest},
	}
	for _, c := range cases {
		body := acceptBody(p, "reaccept")
		c.mut(body)
		rec := f.accept(t, body)
		if rec.Code != c.status || decodeAPIError(t, rec).Code != c.code {
			t.Errorf("%s: %d %s, want %d %s", c.name, rec.Code, rec.Body.String(), c.status, c.code)
		}
	}
	// Nada do que foi recusado gravou aceite.
	if after := f.pending(t, "pt-BR"); len(after.Documents) != 2 {
		t.Fatalf("aceite recusado resolveu pendência: %+v", after)
	}
}

func TestLegalRoutesNeedASession(t *testing.T) {
	s := &Server{}
	for name, h := range map[string]http.HandlerFunc{"pending": s.pendingLegalHandler, "accept": s.acceptLegalHandler} {
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`)))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s sem sessão: %d", name, rec.Code)
		}
	}
}

// Outra conta não aceita nem lê pendência por esta: o dono sai do token.
func TestLegalAcceptIsForTheTokenOwnerOnly(t *testing.T) {
	f := newLegalFixture(t)
	v := f.publish(t, 1, true, time.Now().Add(-time.Hour))
	f.change(t, "terms", v, "pt-BR", "added", "mudança")
	p := f.pending(t, "pt-BR")

	// A outra conta está no mesmo ponto: aceitou as mesmas versões. O corpo, idêntico ao
	// que o dono mandaria, ainda aponta para ele; quem manda é o token.
	other := f.seedPasswordUser(t, f.email("outra"))
	for kind, version := range map[string]int{"terms": f.terms, "privacy": f.privacy} {
		if _, err := f.pool.Exec(context.Background(), `
			INSERT INTO legal_acceptances (user_id, kind, version, locale) VALUES ($1, $2, $3, 'pt-BR')
		`, other, kind, version); err != nil {
			t.Fatal(err)
		}
	}
	body := acceptBody(p, "reaccept")
	body["user_id"] = f.user
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(raw)))
	req.Header.Set("Authorization", bearer(t, other))
	rec := httptest.NewRecorder()
	f.s.acceptLegalHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("o aceite da outra conta, com o token dela: %d %s", rec.Code, rec.Body.String())
	}
	if after := f.pending(t, "pt-BR"); len(after.Documents) != 2 {
		t.Fatal("a pendência do dono foi resolvida pelo token de outra conta")
	}
}

// A vigente é a do português; o corpo, na língua pedida quando existe, e o português
// quando a versão não tem aquela língua.
func TestLegalPendingFallsBackPerVersion(t *testing.T) {
	f := newLegalFixture(t)
	v := f.publish(t, 1, true, time.Now().Add(-time.Hour))
	if _, err := f.pool.Exec(context.Background(), `DELETE FROM legal_documents WHERE version = $1 AND locale = 'en'`, v); err != nil {
		t.Fatal(err)
	}
	p := f.pending(t, "en")
	if len(p.Documents) != 2 || p.Documents[0].Version != v || p.Documents[0].Locale != "pt-BR" {
		t.Fatalf("versão sem inglês tem de sair em português, e ainda pendente: %+v", p.Documents)
	}
}

func TestLegalAcceptBodyOverTheLimit(t *testing.T) {
	f := newLegalFixture(t)
	handler := newRateLimiter(1000, 60e9).wrap(limitBody(authBodyLimit, f.s.acceptLegalHandler))
	body := `{"source":"reaccept","shown_changes":["` + strings.Repeat("a", authBodyLimit) + `"]}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Authorization", bearer(t, f.user))
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusBadRequest || decodeAPIError(t, rec).Code != codeInvalidRequest {
		t.Fatalf("corpo acima do limite: %d %s", rec.Code, rec.Body.String())
	}
}

// Versão publicada para o futuro não se aceita no cadastro: não está no ar.
func TestSignupRefusesAFutureVersion(t *testing.T) {
	f := newLegalFixture(t)
	future := f.publish(t, 1, true, time.Now().Add(24*time.Hour))
	google := fakeVerifier{}
	f.s.social["google"] = google
	addr := f.email("cadastro-futuro")
	f.cleanupEmail(t, addr)
	google["t"] = socialauth.Identity{Subject: "futuro-" + f.tag, Email: addr, EmailVerified: true}
	body := f.signupBody("t")
	body["legal_acceptances"] = []map[string]any{
		{"kind": "terms", "version": future, "locale": "pt-BR"},
		{"kind": "privacy", "version": future, "locale": "pt-BR"},
	}
	expect(t, f.post(t, body), http.StatusConflict, codeLegalVersionOutdated)
}

// O cadastro grava o hash do texto aceito, calculado pelo servidor.
func TestSignupAcceptanceRecordsTheHash(t *testing.T) {
	google := fakeVerifier{}
	f := newSocialFixture(t, google)
	addr := f.email("cadastro-hash")
	f.cleanupEmail(t, addr)
	google["t"] = socialauth.Identity{Subject: "cadastro-hash-" + f.tag, Email: addr, EmailVerified: true}
	userID := sessionUser(t, f.post(t, f.signupBody("t")))

	var sha, source string
	if err := f.pool.QueryRow(context.Background(), `
		SELECT a.body_sha256, a.source FROM legal_acceptances a WHERE a.user_id = $1 AND a.kind = 'terms'
	`, userID).Scan(&sha, &source); err != nil {
		t.Fatal(err)
	}
	var want string
	if err := f.pool.QueryRow(context.Background(), `
		SELECT body_html FROM legal_documents WHERE kind = 'terms' AND version = $1 AND locale = 'pt-BR'
	`, f.terms).Scan(&want); err != nil {
		t.Fatal(err)
	}
	if sha != domain.SHA256Hex(want) || source != "signup" {
		t.Fatalf("cadastro gravou sha=%s source=%s", sha, source)
	}
}
