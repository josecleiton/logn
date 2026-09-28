# Landing page

O site de `logn.sh` e `www.logn.sh`: uma página estática em três línguas, servida por um
Cloudflare Worker com assets estáticos.

| Caminho | Língua |
|---|---|
| `/` | pt-BR (também o `x-default`) |
| `/en/` | en |
| `/es/` | es |

## Como está montado

```
landing/
├── wrangler.toml       rotas logn.sh + www.logn.sh, build, assets, proxy /legal/*
├── build.mjs           gera dist/ — sem dependência, só Node
├── i18n/               cópia da página: pt-BR.toml, en.toml, es.toml
└── src/
    ├── index.html      template ({{chave}} escapa HTML; {{{chave}}} é só para o build)
    ├── 404.html        "Wrong Answer." — um por língua, o Cloudflare serve o mais próximo
    ├── _headers        CSP e demais cabeçalhos dos assets
    ├── worker.js       proxy de /legal/terms e /legal/privacy para o backend
    └── assets/         site.css, site.js, icons/ (favicon SVG, PNG 32, apple-touch-icon)
```

`dist/` não entra no git. As fontes IBM Plex são copiadas de onde o app já as tem
(`ios/…/Resources/Fonts`), para não versionar o mesmo binário duas vezes.

## Uso

```sh
just landing-dev       # http://127.0.0.1:8788
just landing-deploy    # sobe em logn.sh e www.logn.sh
```

Antes do primeiro deploy, a zona `logn.sh` precisa estar na conta Cloudflare (os custom
domains criam DNS e certificado sozinhos) e o proxy legal precisa da origem do backend:

```sh
just landing-legal-origin   # URL do Cloud Run → secret LEGAL_ORIGIN do Worker
```

A URL fica no secret do Worker e não aparece em nenhum arquivo daqui (AGENTS.md, regra 9).
Sem ela, `/legal/*` responde 503 — falha fechada.

Quando o app estiver na loja, o deploy passa a levar o link:

```sh
LOGN_APP_STORE_URL=https://apps.apple.com/app/id… just landing-deploy
```

Sem a variável, a página diz "em breve na App Store" e os botões levam ao fim da página.

## Texto

Toda frase da página está em `i18n/<língua>.toml`, nas três. O build falha se uma
língua tiver chave a mais ou a menos, se alguma vier vazia, ou se o template pedir uma
chave que não existe. Frase nova: chave nas três línguas, `{{grupo.chave}}` no template.

Nomes de técnica, `AC`/`WA`/`TLE`, `PROBLEM F` e trechos de código ficam iguais nas três
línguas de propósito — é o vocabulário de maratona que o app também usa.

## Segurança

- CSP `default-src 'none'` com `'self'` só para script, estilo, fonte e imagem. Não há
  estilo nem script inline; o `site.js` mexe em estilo por CSSOM, que a CSP permite.
- Nada de terceiro: nem Google Fonts, nem analytics. A página não manda o IP de ninguém
  para outro domínio.
- O proxy de `/legal/*` aceita só `GET`/`HEAD`, só os dois caminhos, repassa só `lang`
  (de lista fechada) e `Accept-Language`, e não repassa cookie.

## Conteúdo e marca

Esta pasta é pública como o resto do repositório. O desafio na tela do celular, os
trechos dos formatos e os times do placar são **inventados** — nenhum vem da trilha real
(AGENTS.md, regra 8). O mapa de técnicas é ilustrativo e não reproduz o grafo real.

### Selo da App Store

`src/assets/badges/` guarda o selo oficial "Download on the App Store" (Black lockup,
SVG) do pacote da Apple, sem alteração: `PTBR` para pt-BR, `US-UK` para en e `ESMX` para
es. O espanhol é o do México porque a página usa o espanhol latino — o selo diz
"Descárgalo en el", e o de Espanha diria "Consíguelo en el".

O selo só aparece com `LOGN_APP_STORE_URL` definido, porque a regra da Apple é para app
disponível na loja. Antes disso fica um botão só com texto, sem o logotipo da Apple, que
a Apple não deixa usar fora do selo. A arte não é Apache 2.0 (ver `NOTICE`); não
recolorir, recortar nem editar. Pacote novo sai de
https://tools.applemediaservices.com/app-store/.
