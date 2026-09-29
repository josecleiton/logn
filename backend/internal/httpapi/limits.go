package httpapi

import (
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Tetos de corpo por rota. Não havia nenhum: cabiam 32 MB (o limite do Cloud Run) numa
// senha que ia inteira para o Argon2, ou num sync que virava uma transação só.
//
// O sync sobe a fila offline inteira de uma vez, então o teto dele é folgado: cada
// evento tem uns 300 bytes, e 8 MB passam de vinte mil.
//
// A notificação da App Store traz transação e renovação assinadas, cada uma com a
// cadeia de três certificados: fica na casa das dezenas de KB. O teto é folgado de
// propósito até haver medida de tráfego real; o que ele barra é o corpo sem fim.
const (
	authBodyLimit                 = 16 << 10
	syncBodyLimit                 = 8 << 20
	appStoreNotificationBodyLimit = 256 << 10
)

func limitBody(n int64, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, n)
		next(w, r)
	}
}

// rateLimiter conta pedidos por chave numa janela fixa.
//
// Vive na memória da instância: com várias instâncias do Cloud Run, cada uma conta a
// sua parte. É a primeira barreira, não a única. O que protege a conta de verdade fica
// no banco (tentativas por OTP, intervalo entre envios), e isso vale entre instâncias.
type rateLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string]*rateWindow
}

type rateWindow struct {
	start time.Time
	count int
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{limit: limit, window: window, hits: map[string]*rateWindow{}}
	go rl.sweep()
	return rl
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	w, ok := rl.hits[key]
	if !ok || now.Sub(w.start) >= rl.window {
		rl.hits[key] = &rateWindow{start: now, count: 1}
		return true
	}
	w.count++
	return w.count <= rl.limit
}

// sweep joga fora as janelas vencidas, senão o mapa cresce com cada IP que já passou.
func (rl *rateLimiter) sweep() {
	for range time.Tick(rl.window) {
		rl.mu.Lock()
		now := time.Now()
		for k, w := range rl.hits {
			if now.Sub(w.start) >= rl.window {
				delete(rl.hits, k)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *rateLimiter) wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !rl.allow(requestIP(r)) {
			w.Header().Set("Retry-After", "60")
			writeError(w, http.StatusTooManyRequests, codeRateLimited)
			return
		}
		next(w, r)
	}
}

// requestIP é a chave do rate limit. Atrás do proxy, o IP do jogador que a verificação
// de origem confirmou; sem ela, clientIP, que atrás do proxy seria o nó da borda e
// poria todo mundo daquele nó no mesmo balde.
func requestIP(r *http.Request) string {
	if ip, ok := r.Context().Value(verifiedClientIPKey{}).(string); ok {
		return ip
	}
	return clientIP(r)
}

// clientIP devolve o IP da conexão que chegou ao Cloud Run — o jogador sem proxy, o
// nó da borda com proxy.
//
// No Cloud Run o `RemoteAddr` é o front-end do Google, igual para todo mundo. O IP do
// cliente vem no `X-Forwarded-For`, e só a entrada mais à direita é confiável: ela foi
// escrita pela infraestrutura. As da esquerda o cliente escreve o que quiser. Fora do
// Cloud Run o cabeçalho não é confiável e fica o `RemoteAddr`.
func clientIP(r *http.Request) string {
	if os.Getenv("K_SERVICE") != "" {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			return strings.TrimSpace(parts[len(parts)-1])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
