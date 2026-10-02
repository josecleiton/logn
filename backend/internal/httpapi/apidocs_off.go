//go:build !dev

package httpapi

import "net/http"

// registerAPIDocs não faz nada no binário de produção. A documentação da API só existe
// no de desenvolvimento, com `-tags dev` (apidocs_dev.go, ADR 0025).
func registerAPIDocs(*http.ServeMux) error { return nil }
