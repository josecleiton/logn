package main

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
)

// originVerifier bloqueia requisições que não vieram do proxy confiável na frente do
// serviço (hoje Cloudflare; poderia ser outro WAF amanhã sem mudar uma linha aqui,
// só a env). Checa duas coisas, e as duas têm que bater: o IP de origem contra uma
// lista de CIDRs, e um header com segredo que só o proxy injeta. IP sozinho não
// basta (se o segredo vazar, dá para forjar X-Forwarded-For); segredo sozinho não
// basta (se vazar, ainda precisa vir de dentro do range confiável).
//
// Falha fechada: sem ORIGIN_TRUSTED_CIDRS/ORIGIN_SHARED_SECRET em produção, o
// servidor nem sobe — mesmo padrão do JWT_SECRET em main.go.
//
// Atrás do proxy, o IP que o Cloud Run vê (clientIP) é o da borda, compartilhado por
// quem cai no mesmo nó. O do jogador vem num header que o proxy escreve por cima do
// que o cliente mandar (CF-Connecting-IP na Cloudflare), e só vale depois que o pedido
// provou vir do proxy: sem isso, qualquer um escolhe o próprio IP.
type originVerifier struct {
	trustedCIDRs   []*net.IPNet
	headerName     string
	headerValue    string
	clientIPHeader string
}

// verifiedClientIPKey guarda no contexto o IP do jogador informado pelo proxy. Só
// wrap grava, e só em pedido que passou por allowed.
type verifiedClientIPKey struct{}

func newOriginVerifierFromEnv() (*originVerifier, error) {
	cidrsRaw := os.Getenv("ORIGIN_TRUSTED_CIDRS")
	secret := os.Getenv("ORIGIN_SHARED_SECRET")
	if cidrsRaw == "" || secret == "" {
		return nil, fmt.Errorf("ORIGIN_TRUSTED_CIDRS e ORIGIN_SHARED_SECRET são obrigatórios")
	}

	headerName := os.Getenv("ORIGIN_SECRET_HEADER")
	if headerName == "" {
		headerName = "X-Origin-Verify"
	}

	clientIPHeader := os.Getenv("ORIGIN_CLIENT_IP_HEADER")
	if clientIPHeader == "" {
		clientIPHeader = "CF-Connecting-IP"
	}

	var cidrs []*net.IPNet
	for _, raw := range strings.Split(cidrsRaw, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		_, ipnet, err := net.ParseCIDR(raw)
		if err != nil {
			return nil, fmt.Errorf("CIDR inválido em ORIGIN_TRUSTED_CIDRS: %q: %w", raw, err)
		}
		cidrs = append(cidrs, ipnet)
	}
	if len(cidrs) == 0 {
		return nil, fmt.Errorf("ORIGIN_TRUSTED_CIDRS não tem nenhum CIDR válido")
	}

	return &originVerifier{
		trustedCIDRs:   cidrs,
		headerName:     headerName,
		headerValue:    secret,
		clientIPHeader: clientIPHeader,
	}, nil
}

func (v *originVerifier) allowed(r *http.Request) bool {
	ip := net.ParseIP(clientIP(r))
	if ip == nil {
		return false
	}

	fromTrustedNet := false
	for _, cidr := range v.trustedCIDRs {
		if cidr.Contains(ip) {
			fromTrustedNet = true
			break
		}
	}
	if !fromTrustedNet {
		return false
	}

	got := r.Header.Get(v.headerName)
	return subtle.ConstantTimeCompare([]byte(got), []byte(v.headerValue)) == 1
}

// wrap protege o handler inteiro. exempt decide o que fica de fora — sondas do Cloud
// Run e chamadas de serviço a serviço não passam pelo proxy e não têm como carregar o
// header secreto; essas já têm autenticação própria ou nem são alcançáveis de fora.
//
// Pedido verificado leva adiante o IP do jogador (ver verifiedClientIPKey). Isento que
// não veio do proxy segue sem ele, e o rate limit cai no clientIP.
func (v *originVerifier) wrap(next http.Handler, exempt func(*http.Request) bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v.allowed(r) {
			if ip := net.ParseIP(strings.TrimSpace(r.Header.Get(v.clientIPHeader))); ip != nil {
				r = r.WithContext(context.WithValue(r.Context(), verifiedClientIPKey{}, ip.String()))
			}
			next.ServeHTTP(w, r)
			return
		}
		if exempt(r) {
			next.ServeHTTP(w, r)
			return
		}
		writeError(w, http.StatusForbidden, codeOriginNotVerified)
	})
}
