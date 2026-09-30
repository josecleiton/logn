// Só entra aqui o que o `run_worker_first` manda: `/legal/*`. O resto do site sai
// direto dos assets.
//
// As páginas de termos e privacidade vivem no backend (`backend/internal/httpapi/legal_pages.go`), que
// já as serve com CSP, allowlist de tags e as três línguas. O Worker redireciona
// `logn.sh/legal/...` para elas, sem copiar texto jurídico para cá — uma fonte, não duas.
//
// Era proxy. Não funciona com a verificação de origem: a Cloudflare não roda a Transform
// Rule que põe o `X-Origin-Verify` nos pedidos que um Worker faz para um host da própria
// zona, e o backend recusava todos com `origin_not_verified`. Mandar o cabeçalho daqui
// exigiria uma terceira cópia do segredo de origem (ADR 0012).

// Lista fechada: nada de repassar caminho cru para o backend.
const LEGAL_PATHS = new Set(["/legal/terms", "/legal/privacy"]);
const LANGS = new Set(["pt-BR", "en", "es"]);

export default {
  async fetch(request, env) {
    const url = new URL(request.url);

    if (!LEGAL_PATHS.has(url.pathname)) {
      return env.ASSETS.fetch(request);
    }
    if (request.method !== "GET" && request.method !== "HEAD") {
      return new Response(null, { status: 405, headers: { Allow: "GET, HEAD" } });
    }
    if (!env.LEGAL_ORIGIN) {
      // Falha fechada: sem origem configurada, não inventa destino.
      return new Response(null, { status: 503 });
    }

    // O destino é a origem configurada, com o caminho da lista e só o `lang` de lista
    // fechada: nada do pedido vira parte do endereço de outro jeito.
    const target = new URL(url.pathname, env.LEGAL_ORIGIN);
    const lang = url.searchParams.get("lang");
    if (lang && LANGS.has(lang)) target.searchParams.set("lang", lang);

    // 302, não 301: o navegador não grava o destino para sempre, e dá para voltar a
    // servir daqui sem esperar cache nenhum expirar.
    return new Response(null, {
      status: 302,
      headers: {
        Location: target.toString(),
        "Cache-Control": "no-store",
        "Referrer-Policy": "no-referrer",
      },
    });
  },
};
