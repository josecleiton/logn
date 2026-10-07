// A página de `app/verify` e `app/reset-password` (ADR 0028), para quando o botão do
// e-mail abre no navegador: no computador, no iPhone, ou num Android que não verificou o
// App Link. O "Abrir no LogN" leva ao `logn://` com os parâmetros, que vêm no fragmento.
(() => {
  const open = document.querySelector("[data-app]");
  const hash = new URLSearchParams(location.hash.slice(1));
  // O código sai da barra de endereço e do histórico, mesmo com o link pela metade.
  if (location.hash) history.replaceState(null, "", location.pathname);
  if (!open || !hash.get("code") || !hash.get("email")) return;

  // Só os três que o app lê, e nada que tenha vindo junto.
  const params = new URLSearchParams();
  for (const key of ["code", "email", "purpose"]) {
    if (hash.has(key)) params.set(key, hash.get(key));
  }
  open.href = `logn://${open.dataset.app}?${params}`;
  open.hidden = false;
})();
