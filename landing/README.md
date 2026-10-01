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
├── wrangler.toml       rotas logn.sh + www.logn.sh, build, assets, redirect /legal/*
├── build.mjs           gera dist/ — sem dependência, só Node
├── i18n/               cópia da página: pt-BR.toml, en.toml, es.toml
└── src/
    ├── index.html      template ({{chave}} escapa HTML; {{{chave}}} é só para o build)
    ├── 404.html        "Wrong Answer." — um por língua, o Cloudflare serve o mais próximo
    ├── account-delete.html  /account/delete/, a URL de exclusão da ficha do Google Play
    ├── waitlist.html   as quatro páginas de volta da lista de espera do iPhone
    ├── _headers        CSP e demais cabeçalhos dos assets
    ├── worker.js       redirect de /legal/terms e /legal/privacy para o backend
    ├── .well-known/    assetlinks.json, o Digital Asset Links do app Android (Play App Signing)
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
domains criam DNS e certificado sozinhos) e o redirect legal precisa do destino:

```sh
just landing-legal-origin https://api.logn.sh   # → secret LEGAL_ORIGIN do Worker
```

É o domínio público da API, nunca a URL `.run.app`: ela recusa `/legal` pela verificação
de origem, e apareceria no endereço. Sem o secret, `/legal/*` responde 503 — falha fechada.

O lançamento é no Google Play (ADR 0022). Quando a ficha estiver publicada, o deploy
passa a levar o link:

```sh
LOGN_PLAY_STORE_URL='https://play.google.com/store/apps/details?id=…' just landing-deploy
```

Sem a variável, a página diz "em breve no Google Play" e os botões levam ao fim da
página. `LOGN_APP_STORE_URL` põe o selo da App Store ao lado e fecha a lista de espera
do iPhone; enquanto o app não está lá, fica vazio.

### Lista de espera do iPhone

```sh
LOGN_API_ORIGIN=https://api.logn.sh just landing-deploy
```

Com o domínio da API, a seção do fim ganha o formulário "Tem iPhone?", e o `form-action`
da CSP passa a aceitar esse domínio e a própria landing, para onde a API responde com
303, e nada mais. Sem a variável, o formulário não sai e a
CSP fica com `form-action 'none'`. **Só defina depois que a rota `POST
/api/v1/waitlist` estiver no ar e a política de privacidade cobrir a lista.**

O backend responde com `303` para uma destas páginas, na língua do formulário:

| Caminho | Quando |
|---|---|
| `/waitlist/thanks/` | formulário recebido (a mesma resposta para e-mail novo e repetido) |
| `/waitlist/confirmed/` | link de confirmação aberto |
| `/waitlist/left/` | link de saída aberto |
| `/waitlist/error/` | link vencido, pedido recusado ou limite de pedidos |

Com o prefixo `/en/` e `/es/` nas outras línguas. Saem com `noindex` e fora do sitemap.

### Exclusão de conta

`/account/delete/` (e `/en/…`, `/es/…`) é a URL que a ficha do Google Play pede em
"Segurança dos dados". Repete a seção 10 da política e o caminho por
`contact@logn.sh`; mudou a política, muda aqui.

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
  para outro domínio. A única saída é o formulário da lista de espera, que posta na
  própria API, sem JS, e só existe com `LOGN_API_ORIGIN`.
- O redirect de `/legal/*` aceita só `GET`/`HEAD`, só os dois caminhos, e leva só o
  `lang` de lista fechada. Não é proxy: a Cloudflare não põe o cabeçalho de origem nos
  pedidos do Worker para a própria zona (ADR 0012).

## Conteúdo e marca

Esta pasta é pública como o resto do repositório. O desafio na tela do celular, os
trechos dos formatos e os times do placar são **inventados** — nenhum vem da trilha real
(AGENTS.md, regra 8). O mapa de técnicas é ilustrativo e não reproduz o grafo real.

### Selo do Google Play

Não está no repositório ainda: a regra do Google é a mesma da Apple, selo só para app
disponível. Quando a ficha for publicada, baixe o selo oficial "Disponível no Google
Play" em PNG, sem alteração, de https://play.google.com/intl/pt-BR/badges/ (e das
páginas `en` e `es-419`), e salve como `src/assets/badges/google-play-<pt-BR|en|es>.png`.
O build falha se `LOGN_PLAY_STORE_URL` estiver definido e um deles faltar, e tira a
proporção do próprio arquivo.

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
