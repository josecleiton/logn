package httpapi

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/josecleiton/logn/backend/internal/domain"
	"github.com/josecleiton/logn/backend/internal/locale"
)

// nodesResponse é o corpo de GET /api/v1/nodes. `Origins` viaja junto (ADR 0011): o
// cartão de origem é conteúdo, como os nós, e não faz sentido buscá-lo numa rota à
// parte só para servir um selo que aparece dentro da trilha.
type nodesResponse struct {
	Nodes   []domain.SkillNode  `json:"nodes"`
	Origins []domain.OriginCard `json:"origins"`
}

// O método já vem filtrado pela rota: `GET /api/v1/nodes` aceita também HEAD, que uma
// guarda de GET aqui dentro recusava com 405.
func (s *Server) getNodesHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.optionalAccount(w, r)
	if !ok {
		return
	}
	lang := locale.Negotiate(r)
	nodes, err := s.repo.GetSkillNodes(r.Context(), lang, userID)
	if err != nil {
		log.Printf("nós não lidos: locale=%s erro=%v", lang, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}
	origins, err := s.repo.GetOriginCards(r.Context(), lang)
	if err != nil {
		log.Printf("origens não lidas: locale=%s erro=%v", lang, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}

	writeAccountContentJSON(w, lang, userID, nodesResponse{Nodes: nodes, Origins: origins})
}

func (s *Server) getUserProgressHandler(w http.ResponseWriter, r *http.Request) {
	// Era um UUID fixo, escrito à mão, que não pertencia a ninguém: a rota respondia
	// `null` para todo mundo e nenhum cliente a chamava.
	userID, ok := s.authenticate(w, r)
	if !ok {
		return
	}

	stats, err := s.repo.GetUserStats(r.Context(), userID)
	if err != nil {
		log.Printf("progresso não lido: user=%s erro=%v", userID, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
