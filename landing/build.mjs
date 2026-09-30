#!/usr/bin/env node
// Gera `dist/` — a landing page nas três línguas — a partir de `src/` e `i18n/`.
//
//   node build.mjs                                     # "em breve no Google Play"
//   LOGN_PLAY_STORE_URL=https://play.google.com/store/apps/details?id=... node build.mjs
//   LOGN_APP_STORE_URL=https://apps.apple.com/...      # o selo da App Store ao lado
//   LOGN_API_ORIGIN=https://api.example.com            # a lista de espera do iPhone
//
// Sem dependência: o TOML de `i18n/` é o subconjunto plano que este arquivo lê
// (seções `[grupo]` e `chave = "texto"`), e qualquer outra coisa é erro, não palpite.
// O build falha se uma língua tiver chave a mais ou a menos, ou se o template pedir
// uma chave que não existe — texto faltando não sobe para produção.

import { createHash } from "node:crypto";
import { copyFileSync, existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = dirname(fileURLToPath(import.meta.url));
const REPO = join(ROOT, "..");
const DIST = join(ROOT, "dist");
const ORIGIN = "https://logn.sh";

// A primeira é a da raiz e a fonte da verdade das chaves.
const LOCALES = [
  { tag: "pt-BR", dir: "", og: "pt_BR", short: "PT" },
  { tag: "en", dir: "en/", og: "en_US", short: "EN" },
  { tag: "es", dir: "es/", og: "es_ES", short: "ES" },
];

// Fontes e símbolo vêm de onde o app já os tem; não duplicamos binário no git.
const FONTS_DIR = join(REPO, "ios/LogNiOS/LogNiOS/Resources/Fonts");
const FONTS = [
  "IBMPlexSans-Regular.ttf", "IBMPlexSans-Medium.ttf", "IBMPlexSans-SemiBold.ttf",
  "IBMPlexMono-Regular.ttf", "IBMPlexMono-Medium.ttf",
];
// Favicon do design system: o balão no quadrado escuro, em SVG, PNG 32 e 180 (iOS).
const ICONS = ["favicon.svg", "favicon-32.png", "apple-touch-icon.png"];

// As lojas, na ordem em que aparecem. A primeira é a do lançamento (ADR 0022): sem link,
// ela mostra "em breve"; as outras só aparecem quando têm link.
//
// Selo é arte de terceiro, como veio do pacote oficial, e não se edita. Só sobe para
// `dist/` quando a loja tem link, porque as duas regras de marca são para app disponível:
// antes disso fica o botão só com texto, sem logotipo. O da Apple é o Black lockup em
// PTBR, US-UK e ESMX (o espanhol da página é o latino); o do Google Play entra em
// `src/assets/badges/` quando a ficha for publicada, e o build falha se faltar.
const STORES = [
  {
    id: "play", env: "LOGN_PLAY_STORE_URL", name: "Google Play",
    url: /^https:\/\/play\.google\.com\/store\/apps\/details\?id=[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$/i,
    example: "https://play.google.com/store/apps/details?id=...",
    badge: (tag) => `/assets/badges/google-play-${tag}.png`,
    over: { live: "cta.play_live", soon: "cta.play_soon" },
  },
  {
    id: "apple", env: "LOGN_APP_STORE_URL", name: "App Store",
    url: /^https:\/\/apps\.apple\.com\/[a-z0-9/._-]+$/i,
    example: "https://apps.apple.com/...",
    badge: (tag) => `/assets/badges/app-store-${tag}.svg`,
    over: { live: "cta.apple_live" },
  },
];
const BADGE_HEIGHT = 56;

// Largura e altura do selo, do próprio arquivo: o `viewBox` do SVG ou o IHDR do PNG.
function badgeSize(file) {
  const body = readFileSync(file);
  if (file.endsWith(".png")) {
    if (body.toString("latin1", 1, 4) !== "PNG") fail(`${file}: não é PNG`);
    return { w: body.readUInt32BE(16), h: body.readUInt32BE(20) };
  }
  const box = /viewBox="[\d.]+ [\d.]+ ([\d.]+) ([\d.]+)"/.exec(body.toString("utf8"));
  if (!box) fail(`${file}: sem viewBox`);
  return { w: Number(box[1]), h: Number(box[2]) };
}

// Com o app na loja, o selo oficial com o link. Antes disso, o botão só com texto.
function storeButton(store, over, home, tag) {
  if (store.link) {
    const { w, h } = store.sizes.get(tag);
    const width = Math.round(BADGE_HEIGHT * (w / h));
    return `<a class="store-badge" href="${escapeHtml(store.link)}">`
      + `<img src="${store.badge(tag)}" alt="${escapeHtml(`${over} ${store.name}`)}" width="${width}" height="${BADGE_HEIGHT}"></a>`;
  }
  return `<a class="store" href="${home}#fim">`
    + `<span class="store-text"><span class="store-over">${escapeHtml(over)}</span>`
    + `<span class="store-name">${escapeHtml(store.name)}</span></span></a>`;
}

// A lista de espera do iPhone (ADR 0022): só existe enquanto o app não está na App
// Store e só com o endereço da API, porque é ela quem recebe o formulário. Sem JS: um
// `<form>` puro, e o backend responde com um 303 para uma das páginas de `waitlist/`.
// O campo `website` é isca: fica fora da tela e do leitor, e só robô o preenche.
function waitlistForm(origin, s, tag) {
  return `<form class="waitlist" method="post" action="${escapeHtml(origin)}/api/v1/waitlist">`
    + `<h3 class="waitlist-title">${escapeHtml(s("waitlist.title"))}</h3>`
    + `<p class="waitlist-lead">${escapeHtml(s("waitlist.lead"))}</p>`
    + `<input type="hidden" name="locale" value="${tag}">`
    + `<div class="waitlist-row">`
    + `<label class="sr" for="waitlist-email">${escapeHtml(s("waitlist.label"))}</label>`
    + `<input id="waitlist-email" class="waitlist-input mono" type="email" name="email" required maxlength="254"`
    + ` autocomplete="email" placeholder="${escapeHtml(s("waitlist.placeholder"))}">`
    + `<button class="btn-accent waitlist-send" type="submit">${escapeHtml(s("waitlist.send"))}</button>`
    + `</div>`
    + `<div class="hp" aria-hidden="true"><input type="text" name="website" tabindex="-1" autocomplete="off"></div>`
    + `<p class="note waitlist-note">${escapeHtml(s("waitlist.note"))} `
    + `<a href="/legal/privacy?lang=${tag}">${escapeHtml(s("footer.privacy"))}</a></p>`
    + `</form>`;
}

// Placar de exemplo. Times inventados — nada de instituição real (AGENTS.md, regra 8).
// Célula: `+t/m` aceito na tentativa t+1 no minuto m, `-n` n erros, `?` congelado, `.` vazio.
const BOARD = [
  ["1", "Big Theta", "6", "412", "+/12 +/31 +1/58 +/74 ? +2/141 +/166"],
  ["2", "Off by One", "5", "388", "+/9 +/40 +/77 +3/119 . ? +/173"],
  ["3", null, "5", "402", "+/14 +1/36 +/63 . ? +/128 +2/180"],
  ["4", "Segfault", "4", "301", "+/22 +/49 -2 +/98 ? . +1/190"],
  ["5", "Mod 1e9+7", "3", "244", "+/18 -3 +/71 . ? . +/203"],
];

function fail(message) {
  console.error(`landing: ${message}`);
  process.exit(1);
}

function parseToml(path) {
  const out = new Map();
  let section = null;
  readFileSync(path, "utf8").split("\n").forEach((raw, i) => {
    const line = raw.trim();
    const where = `${path}:${i + 1}`;
    if (line === "" || line.startsWith("#")) return;
    const header = /^\[([a-z_]+)\]$/.exec(line);
    if (header) { section = header[1]; return; }
    const pair = /^([a-z_]+)\s*=\s*("(?:[^"\\]|\\.)*")\s*(?:#.*)?$/.exec(line);
    if (!pair) fail(`${where}: só \`chave = "texto"\` e \`[seção]\` são aceitos`);
    if (!section) fail(`${where}: chave fora de seção`);
    const key = `${section}.${pair[1]}`;
    if (out.has(key)) fail(`${where}: \`${key}\` repetida`);
    out.set(key, JSON.parse(pair[2]));
  });
  return out;
}

const escapeHtml = (s) => String(s)
  .replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")
  .replace(/"/g, "&quot;").replace(/'/g, "&#39;");

function render(template, values, name) {
  const out = template
    .replace(/\{\{\{\s*([a-z_.]+)\s*\}\}\}/g, (_, key) => {
      if (!values.has(key)) fail(`${name}: \`{{{${key}}}}\` sem valor`);
      return values.get(key);
    })
    .replace(/\{\{\s*([a-z_.]+)\s*\}\}/g, (_, key) => {
      if (!values.has(key)) fail(`${name}: \`{{${key}}}\` sem valor`);
      return escapeHtml(values.get(key));
    });
  if (out.includes("{{")) fail(`${name}: sobrou marcador sem substituir`);
  return out;
}

function boardCell(token) {
  if (token === ".") return `<td><span class="sub"></span></td>`;
  if (token === "?") return `<td><span class="sub frz"><b>?</b><small>frz</small></span></td>`;
  if (token.startsWith("-")) return `<td><span class="sub err"><b>−${escapeHtml(token.slice(1))}</b></span></td>`;
  const [tries, minute] = token.slice(1).split("/");
  return `<td><span class="sub ok"><b>+${escapeHtml(tries)}</b><small>${escapeHtml(minute)}</small></span></td>`;
}

function boardRows(you) {
  return BOARD.map(([rank, team, ac, pen, cells]) => {
    const me = team === null;
    return `<tr${me ? ` class="me"` : ""}><td class="rank">${rank}</td>`
      + `<td class="team">${escapeHtml(me ? you : team)}</td>`
      + `<td class="num">${ac}</td><td class="num pen">${pen}</td>`
      + cells.split(" ").map(boardCell).join("") + `</tr>`;
  }).join("\n          ");
}

for (const store of STORES) {
  store.link = process.env[store.env] || null;
  if (store.link && !store.url.test(store.link)) fail(`${store.env} tem de ser um link ${store.example}`);
}

// Só o domínio público da API, sem caminho. Nunca a URL `.run.app`, que recusa pedido
// que não passou pela Cloudflare (ADR 0012).
function apiOrigin() {
  const origin = process.env.LOGN_API_ORIGIN;
  if (!origin) return null;
  if (!/^https:\/\/[a-z0-9-]+(\.[a-z0-9-]+)+$/i.test(origin) || /\.run\.app$/i.test(origin)) {
    fail("LOGN_API_ORIGIN tem de ser o domínio público da API, https://..., sem caminho e sem .run.app");
  }
  return origin;
}

// --- Catálogo -------------------------------------------------------------

const catalogs = LOCALES.map((locale) => ({
  locale,
  strings: parseToml(join(ROOT, "i18n", `${locale.tag}.toml`)),
}));
const reference = catalogs[0];
for (const { locale, strings } of catalogs.slice(1)) {
  const missing = [...reference.strings.keys()].filter((k) => !strings.has(k));
  const extra = [...strings.keys()].filter((k) => !reference.strings.has(k));
  if (missing.length) fail(`${locale.tag}.toml não tem: ${missing.join(", ")}`);
  if (extra.length) fail(`${locale.tag}.toml tem a mais: ${extra.join(", ")}`);
  for (const [key, value] of strings) {
    if (value.trim() === "") fail(`${locale.tag}.toml: \`${key}\` vazia`);
  }
}

// --- Páginas --------------------------------------------------------------

// CSS e JS saem com o hash do conteúdo no nome, e o `_headers` os serve como
// imutáveis: deploy novo troca o link, e ninguém fica com o estilo da versão anterior.
function hashed(source, name) {
  const body = readFileSync(join(ROOT, source));
  const digest = createHash("sha256").update(body).digest("hex").slice(0, 10);
  const [base, ext] = name.split(".");
  return { body, path: `/assets/${base}.${digest}.${ext}` };
}
const css = hashed("src/assets/site.css", "site.css");
const js = hashed("src/assets/site.js", "site.js");

const api = apiOrigin();
const live = STORES.filter((store) => store.link);
const [launch] = STORES;
const apple = STORES.find((store) => store.id === "apple");
// A lista de espera é do iPhone: sai quando a App Store entra, e só existe com a API.
const waitlist = api && !apple.link;

for (const store of live) {
  store.sizes = new Map(LOCALES.map(({ tag }) => {
    const file = join(ROOT, "src", store.badge(tag));
    if (!existsSync(file)) fail(`${store.env} definido, mas falta o selo oficial em src${store.badge(tag)}`);
    return [tag, badgeSize(file)];
  }));
}

const template = (name) => readFileSync(join(ROOT, "src", name), "utf8");
const indexTemplate = template("index.html");
const notFoundTemplate = template("404.html");
const deleteTemplate = template("account-delete.html");
const waitlistTemplate = template("waitlist.html");

// As páginas de volta do formulário. O selo é o veredito de maratona, igual nas três
// línguas, como o `404 · WA`.
const WAITLIST_STATES = [
  { id: "thanks", verdict: "PENDING", tone: "t-info" },
  { id: "confirmed", verdict: "AC", tone: "t-ok" },
  { id: "left", verdict: "OK", tone: "t-ok" },
  { id: "error", verdict: "WA", tone: "t-err" },
];

rmSync(DIST, { recursive: true, force: true });
mkdirSync(join(DIST, "assets"), { recursive: true });
mkdirSync(join(DIST, "fonts"), { recursive: true });

// `path` é o caminho depois do prefixo da língua: "" para o início, "account/delete/".
const alternates = (path) => [
  ...LOCALES.map((l) => `<link rel="alternate" hreflang="${l.tag}" href="${ORIGIN}/${l.dir}${path}">`),
  `<link rel="alternate" hreflang="x-default" href="${ORIGIN}/${path}">`,
].join("\n");

for (const { locale, strings } of catalogs) {
  const s = (key) => strings.get(key);
  const home = `/${locale.dir}`;
  const switcher = (path) => `<nav class="lang" aria-label="${escapeHtml(s("meta.lang_switch"))}">`
    + LOCALES.map((l) => {
      const current = l.tag === locale.tag ? ` aria-current="page"` : "";
      const label = escapeHtml(catalogs.find((c) => c.locale.tag === l.tag).strings.get("meta.lang_name"));
      return `<a href="/${l.dir}${path}" hreflang="${l.tag}" lang="${l.tag}" title="${label}"${current}>${l.short}</a>`;
    }).join("")
    + `</nav>`;

  const values = new Map(strings);
  values.set("page.lang", locale.tag);
  values.set("page.og_locale", locale.og);
  values.set("page.home", home);
  values.set("page.css", css.path);
  values.set("page.js", js.path);
  values.set("page.board_rows", boardRows(s("board.you")));
  // Sem link de loja ainda, os botões levam ao fim da página e dizem "em breve".
  values.set("cta.href", live[0]?.link ?? `${home}#fim`);
  values.set("cta.short", s(live.length ? "cta.short_live" : "cta.short_soon"));
  values.set("page.store", [launch, ...STORES.slice(1).filter((store) => store.link)]
    .map((store) => storeButton(store, s(store.over[store.link ? "live" : "soon"]), home, locale.tag))
    .join(""));
  values.set("cta.note", s(!launch.link ? "cta.note_soon" : apple.link ? "cta.note_all" : "cta.note_live"));
  values.set("page.waitlist", waitlist ? waitlistForm(api, s, locale.tag) : "");

  const write = (path, file, tpl, extra = {}) => {
    const page = new Map(values);
    page.set("page.canonical", `${ORIGIN}${home}${path}`);
    page.set("page.alternates", alternates(path));
    page.set("page.lang_switcher", switcher(path));
    for (const [key, value] of Object.entries(extra)) page.set(key, value);
    const outDir = join(DIST, locale.dir, path);
    mkdirSync(outDir, { recursive: true });
    writeFileSync(join(outDir, file), render(tpl, page, `${locale.tag}/${path}${file}`));
  };
  write("", "index.html", indexTemplate);
  write("", "404.html", notFoundTemplate);
  write("account/delete/", "index.html", deleteTemplate);
  for (const state of WAITLIST_STATES) {
    write(`waitlist/${state.id}/`, "index.html", waitlistTemplate, {
      "page.state_title": s(`waitlist.${state.id}_title`),
      "page.state_lead": s(`waitlist.${state.id}_lead`),
      "page.state_verdict": state.verdict,
      "page.state_tone": state.tone,
    });
  }
}

// O formulário posta na API, e a CSP só deixa se ela estiver no `form-action`. `'self'`
// vai junto porque a API responde com 303 para a landing, e o navegador confere o
// destino do redirect contra o `form-action` também. A troca é na linha da CSP e em
// nenhum outro lugar do arquivo.
const headers = readFileSync(join(ROOT, "src/_headers"), "utf8");
const csp = /^(\s*Content-Security-Policy:.*)form-action 'none'(.*)$/m;
if (!csp.test(headers)) fail("src/_headers: a CSP tem de ter `form-action 'none'` para o build trocar");
writeFileSync(join(DIST, "_headers"), waitlist ? headers.replace(csp, `$1form-action 'self' ${api}$2`) : headers);
writeFileSync(join(DIST, css.path), css.body);
writeFileSync(join(DIST, js.path), js.body);
mkdirSync(join(DIST, "assets/icons"), { recursive: true });
for (const icon of ICONS) {
  copyFileSync(join(ROOT, "src/assets/icons", icon), join(DIST, "assets/icons", icon));
}
mkdirSync(join(DIST, "assets/badges"), { recursive: true });
for (const store of live) {
  for (const { tag } of LOCALES) copyFileSync(join(ROOT, "src", store.badge(tag)), join(DIST, store.badge(tag)));
}
for (const font of FONTS) copyFileSync(join(FONTS_DIR, font), join(DIST, "fonts", font));

// As páginas de `waitlist/` saem com `noindex` e ficam fora do mapa.
const INDEXED = ["", "account/delete/"];
writeFileSync(join(DIST, "robots.txt"), `User-agent: *\nAllow: /\nSitemap: ${ORIGIN}/sitemap.xml\n`);
writeFileSync(join(DIST, "sitemap.xml"), [
  `<?xml version="1.0" encoding="UTF-8"?>`,
  `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">`,
  ...INDEXED.flatMap((path) => LOCALES.map((l) => `  <url><loc>${ORIGIN}/${l.dir}${path}</loc>`
    + LOCALES.map((a) => `<xhtml:link rel="alternate" hreflang="${a.tag}" href="${ORIGIN}/${a.dir}${path}"/>`).join("")
    + `</url>`)),
  `</urlset>`,
  ``,
].join("\n"));

const status = STORES.map((store) => `${store.name} ${store.link ? "no ar" : "em breve"}`).join(", ");
console.log(`landing: ${LOCALES.map((l) => l.tag).join(", ")} → dist/ (${status}; lista de espera ${waitlist ? "aberta" : "fechada"})`);
