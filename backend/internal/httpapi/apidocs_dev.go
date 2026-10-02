//go:build dev

package httpapi

import (
	_ "embed"
	"errors"
	"net/http"
	"os"
)

// A documentação da API, só no binário de desenvolvimento (ADR 0025). O spec sai do
// `just api-docs`, que lê as anotações dos handlers; a interface vem da jsDelivr, com
// versão e hash fixos, para não entrar JS de terceiro no repositório.

//go:embed apidocs/swagger.json
var apiDocsSpec []byte

//go:embed apidocs/init.js
var apiDocsInit []byte

// Versão e SRI do swagger-ui-dist. Trocar a versão é recalcular os dois hashes:
// curl -sL <url> | openssl dgst -sha384 -binary | openssl base64 -A
const (
	swaggerUIBase      = "https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.33.1/"
	swaggerUIBundleSRI = "sha384-ZPehFMQommnnuaZ4rpxgkgTT2DKFVp4hZC/7pLit+9Lek9T1YGSo23eHFbvNkXkw"
	swaggerUICSSSRI    = "sha384-Ov4/wv3j2bmct8cDc5X4ngJZohVPzEmc6uDPH8WeljUxO5vtoykvMEfbu9Vh6RaW"
)

const apiDocsPage = `<!doctype html>
<html lang="pt-BR">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>LogN API</title>
<link rel="stylesheet" href="` + swaggerUIBase + `swagger-ui.css" integrity="` + swaggerUICSSSRI + `" crossorigin="anonymous">
</head>
<body>
<div id="swagger-ui"></div>
<script src="` + swaggerUIBase + `swagger-ui-bundle.js" integrity="` + swaggerUIBundleSRI + `" crossorigin="anonymous"></script>
<script src="/docs/init.js"></script>
</body>
</html>
`

// errDevBuildInProduction barra o binário de desenvolvimento no Cloud Run: a
// documentação nunca pode subir junto com o serviço de verdade.
var errDevBuildInProduction = errors.New("binário de desenvolvimento (-tags dev) não sobe no Cloud Run")

func registerAPIDocs(mux *http.ServeMux) error {
	if os.Getenv("K_SERVICE") != "" {
		return errDevBuildInProduction
	}
	mux.HandleFunc("GET /docs", func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Type", "text/html; charset=utf-8")
		h.Set("Cache-Control", "no-store")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy",
			"default-src 'none'; script-src 'self' https://cdn.jsdelivr.net; style-src https://cdn.jsdelivr.net; img-src 'self' data:; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
		w.Write([]byte(apiDocsPage))
	})
	mux.HandleFunc("GET /docs/init.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Write(apiDocsInit)
	})
	mux.HandleFunc("GET /docs/swagger.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Write(apiDocsSpec)
	})
	return nil
}
