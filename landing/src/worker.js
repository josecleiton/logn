// Só entra aqui o que o `run_worker_first` manda: `/legal/*`. O resto do site sai
// direto dos assets.
//
// As páginas de termos e privacidade vivem no backend (`backend/internal/httpapi/legal_pages.go`), que
// já as serve com CSP, allowlist de tags e as três línguas. O Worker só as expõe em
// `logn.sh/legal/...`, sem copiar texto jurídico para cá — uma fonte, não duas.

// Lista fechada: nada de repassar caminho cru para o backend.
const LEGAL_PATHS = new Set(["/legal/terms", "/legal/privacy"]);
const LANGS = new Set(["pt-BR", "en", "es"]);

// Só estes cabeçalhos da resposta do backend chegam ao navegador. Por allowlist, não
// por remoção: `Set-Cookie`, `Location` ou qualquer coisa que o backend passe a mandar
// um dia não vaza para `logn.sh` sem alguém mexer aqui.
const RESPONSE_HEADERS = [
  "Content-Type",
  "Content-Language",
  "Content-Security-Policy",
  "X-Content-Type-Options",
  "Referrer-Policy",
  "Cache-Control",
];

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

    const upstream = new URL(url.pathname, env.LEGAL_ORIGIN);
    const lang = url.searchParams.get("lang");
    if (lang && LANGS.has(lang)) upstream.searchParams.set("lang", lang);

    const headers = new Headers();
    const acceptLanguage = request.headers.get("Accept-Language");
    if (acceptLanguage) headers.set("Accept-Language", acceptLanguage);

    let response;
    try {
      response = await fetch(upstream, {
        method: request.method,
        headers,
        redirect: "manual",
      });
    } catch {
      return new Response(null, { status: 502 });
    }
    // Redirect do backend carregaria no `Location` o endereço real do Cloud Run, que a
    // origem em secret existe para não mostrar. As duas rotas não redirecionam; se um
    // dia redirecionarem, é erro, não navegação.
    if (response.status >= 300 && response.status < 400) {
      return new Response(null, { status: 502 });
    }

    const out = new Headers();
    for (const name of RESPONSE_HEADERS) {
      const value = response.headers.get(name);
      if (value) out.set(name, value);
    }
    return new Response(response.body, { status: response.status, headers: out });
  },
};
