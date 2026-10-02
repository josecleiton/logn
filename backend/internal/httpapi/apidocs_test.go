package httpapi

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/josecleiton/logn/backend/internal/domain"
)

// O spec da API sai das anotações (`just api-docs`, ADR 0025), e anotação esquecida não
// quebra build nenhum. Este teste lê as rotas do código-fonte e cobra as duas direções:
// rota sem anotação, e anotação de rota que já não existe.
func TestAPIDocsCoverEveryRoute(t *testing.T) {
	raw, err := os.ReadFile("apidocs/swagger.json")
	if err != nil {
		t.Fatalf("spec ausente, rode just api-docs: %v", err)
	}
	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("spec ilegível: %v", err)
	}
	documented := map[string]bool{}
	for path, ops := range spec.Paths {
		for method := range ops {
			documented[strings.ToUpper(method)+" "+path] = true
		}
	}

	registered := registeredRoutes(t)
	if len(registered) == 0 {
		t.Fatal("nenhuma rota lida do código-fonte")
	}
	for route := range registered {
		if !documented[route] {
			t.Errorf("%s não tem anotação no spec; anote o handler e rode just api-docs", route)
		}
	}
	for route := range documented {
		if !registered[route] {
			t.Errorf("o spec documenta %s, que nenhum HandleFunc registra; rode just api-docs", route)
		}
	}
}

// registeredRoutes junta o padrão de todo `Handle` e `HandleFunc` do pacote, fora os testes e as
// rotas da própria documentação. Padrão que o teste não sabe ler é falha, não silêncio:
// uma rota montada de outro jeito escaparia da conferência.
func registeredRoutes(t *testing.T) map[string]bool {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	routes := map[string]bool{}
	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") || strings.HasPrefix(name, "apidocs_") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || (sel.Sel.Name != "HandleFunc" && sel.Sel.Name != "Handle") {
				return true
			}
			patterns, ok := routePatterns(call.Args[0])
			if !ok {
				t.Errorf("%s: padrão de rota que o teste não sabe ler", fset.Position(call.Pos()))
				return true
			}
			for _, p := range patterns {
				routes[p] = true
			}
			return true
		})
	}
	return routes
}

// routePatterns aceita o literal ("GET /ping") e o literal somado à ação da lista de
// espera, que `registerWaitlistRoutes` monta num laço.
func routePatterns(expr ast.Expr) ([]string, bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		s, err := strconv.Unquote(e.Value)
		return []string{s}, err == nil
	case *ast.BinaryExpr:
		lit, ok := e.X.(*ast.BasicLit)
		ident, isIdent := e.Y.(*ast.Ident)
		if !ok || !isIdent || ident.Name != "action" || e.Op != token.ADD {
			return nil, false
		}
		prefix, err := strconv.Unquote(lit.Value)
		if err != nil {
			return nil, false
		}
		return []string{
			prefix + domain.WaitlistActionConfirm,
			prefix + domain.WaitlistActionLeave,
		}, true
	}
	return nil, false
}
