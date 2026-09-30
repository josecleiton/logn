# ADR 0012: Landing page em Cloudflare Worker, fora do backend

## 1. Visão Geral

`logn.sh` não tinha página. O domínio já é o do contato (`contact@logn.sh`) e do remetente de e-mail, e o `welcome.html` aponta para ele, mas quem digitava o endereço não encontrava nada. A landing precisava ficar em `logn.sh` e `www.logn.sh`, em pt-BR, en e es, e linkar os termos e a política que o backend já serve em `/legal/*` (ADR 0008).

## 2. Decisão

**A landing é estática, em `landing/`, servida por um Cloudflare Worker com Static Assets.** O `wrangler.toml` declara os dois hostnames como custom domains; o Cloudflare cuida de DNS e certificado. O `canonical` de toda página aponta para `logn.sh`.

**O build é um script Node sem dependência** (`landing/build.mjs`). Ele lê `landing/i18n/<língua>.toml`, recusa língua com chave a mais, a menos ou vazia, e gera `/`, `/en/` e `/es/` a partir de um template com HTML escapado. `dist/` não entra no git: o `[build]` do Wrangler roda o script a cada deploy. A cópia da landing tem catálogo próprio, e não usa `i18n/keys.toml`, porque é texto de site, não da interface do app, e ninguém a lê pelo `Str.*` gerado.

**O Worker só executa em `/legal/*`** (`run_worker_first`), e redireciona (302) `/legal/terms` e `/legal/privacy` para as páginas do backend no domínio público da API. Aceita só esses dois caminhos e só `GET`/`HEAD`, e leva só o `lang` de lista fechada. O destino fica no secret `LEGAL_ORIGIN` do Worker (`just landing-legal-origin https://api.logn.sh`), nunca a URL `.run.app`, que recusa `/legal` pela verificação de origem.

Era proxy, e nunca funcionou em produção: a Cloudflare não roda a Transform Rule que põe o `X-Origin-Verify` nos pedidos que um Worker faz para um host da própria zona, e o backend recusava todos com `origin_not_verified`. As saídas eram mandar o cabeçalho do Worker, com uma terceira cópia do segredo de origem para manter em dia a cada rotação, ou isentar `/legal/*` da verificação, afrouxando uma defesa (regra 9). O redirect não precisa de segredo; o custo é o endereço mostrar `api.logn.sh`.

**Nada de terceiro na página.** Fontes, CSS e JS saem do próprio domínio; não há Google Fonts nem analytics. Sem script nem estilo inline, a CSP fecha em `'self'`.

**Wrangler roda por `npx`, sem entrar em `package.json` nem em lockfile.** É ferramenta de deploy, como `gcloud` e `terraform`, não dependência de código que vá para o cliente ou para o servidor.

## 3. Alternativas descartadas

* **Servir a landing pelo backend Go, no Cloud Run.** Mistura site de marketing com a API: cada ajuste de texto vira deploy do backend, e o tráfego do site passa a contar no mesmo serviço, com as mesmas instâncias, que o `/sync`.
* **Cloudflare Pages.** Faz o mesmo, mas o Worker com assets é o caminho que o Cloudflare mantém hoje para site novo, e o proxy de `/legal/*` sairia do mesmo jeito.
* **Copiar termos e política para a landing.** Seriam duas fontes de texto jurídico, e a do backend já é versionada no banco, com aceite por versão (ADR 0008).
* **Usar o catálogo do app (`i18n/keys.toml`).** O gerador emite Swift e Kotlin, e o texto de venda nunca aparece dentro do app; misturar os dois só incharia o `Str.*` com chaves que nenhuma tela usa.

## 4. Consequências

* Deploy: `just landing-deploy`. A zona `logn.sh` precisa estar na conta Cloudflare, e o secret `LEGAL_ORIGIN` configurado antes do primeiro deploy; sem ele, `/legal/*` responde 503. Ele ficou sem configurar do lançamento até 2026-09-30, e `logn.sh/legal/*` respondeu 503 nesse tempo.
* O selo oficial da App Store (`landing/src/assets/badges/`) é arte da Apple, fora da Apache 2.0 (ver `NOTICE`), e só aparece quando o deploy leva `LOGN_APP_STORE_URL`.
* Tudo em `landing/` é público. A tela do app mostrada na página usa desafio inventado, e o placar usa times fictícios (regra 8).
