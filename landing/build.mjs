#!/usr/bin/env node
// Gera `dist/` — a landing page nas três línguas — a partir de `src/` e `i18n/`.
//
//   node build.mjs                       # "em breve na App Store"
//   LOGN_APP_STORE_URL=https://apps.apple.com/... node build.mjs   # "baixar na"
//
// Sem dependência: o TOML de `i18n/` é o subconjunto plano que este arquivo lê
// (seções `[grupo]` e `chave = "texto"`), e qualquer outra coisa é erro, não palpite.
// O build falha se uma língua tiver chave a mais ou a menos, ou se o template pedir
// uma chave que não existe — texto faltando não sobe para produção.

import { createHash } from "node:crypto";
import { copyFileSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
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
const FAVICON = join(REPO, "docs/design_system/brand/logn-symbol-accent.svg");

// Selo oficial da Apple, Black lockup, como veio do pacote de Apple Marketing Resources:
// PTBR, US-UK e ESMX (o espanhol da página é o latino). Arte de terceiro, não se edita;
// proporção 119.66 × 40.
const BADGE_RATIO = 119.66407 / 40;
const BADGE_HEIGHT = 56;
const badgePath = (tag) => `/assets/badges/app-store-${tag}.svg`;

// Com o app na loja, o selo oficial com o link. Antes disso a regra da Apple não deixa
// usar o "Download on the App Store", e fica o botão só com texto, sem o logotipo.
function storeButton(store, over, home, tag) {
  if (store) {
    const alt = `${over} App Store`;
    const width = Math.round(BADGE_HEIGHT * BADGE_RATIO);
    return `<a class="store-badge" href="${escapeHtml(store)}">`
      + `<img src="${badgePath(tag)}" alt="${escapeHtml(alt)}" width="${width}" height="${BADGE_HEIGHT}"></a>`;
  }
  return `<a class="store" href="${home}#fim">`
    + `<span class="store-text"><span class="store-over">${escapeHtml(over)}</span>`
    + `<span class="store-name">App Store</span></span></a>`;
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

function appStoreUrl() {
  const url = process.env.LOGN_APP_STORE_URL;
  if (!url) return null;
  if (!/^https:\/\/apps\.apple\.com\/[a-z0-9/._-]+$/i.test(url)) {
    fail("LOGN_APP_STORE_URL tem de ser um link https://apps.apple.com/...");
  }
  return url;
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

const store = appStoreUrl();
const indexTemplate = readFileSync(join(ROOT, "src/index.html"), "utf8");
const notFoundTemplate = readFileSync(join(ROOT, "src/404.html"), "utf8");

rmSync(DIST, { recursive: true, force: true });
mkdirSync(join(DIST, "assets"), { recursive: true });
mkdirSync(join(DIST, "fonts"), { recursive: true });

const alternates = [
  ...LOCALES.map((l) => `<link rel="alternate" hreflang="${l.tag}" href="${ORIGIN}/${l.dir}">`),
  `<link rel="alternate" hreflang="x-default" href="${ORIGIN}/">`,
].join("\n");

for (const { locale, strings } of catalogs) {
  const s = (key) => strings.get(key);
  const home = `/${locale.dir}`;
  const switcher = `<nav class="lang" aria-label="${escapeHtml(s("meta.lang_switch"))}">`
    + LOCALES.map((l) => {
      const current = l.tag === locale.tag ? ` aria-current="page"` : "";
      const label = escapeHtml(catalogs.find((c) => c.locale.tag === l.tag).strings.get("meta.lang_name"));
      return `<a href="/${l.dir}" hreflang="${l.tag}" lang="${l.tag}" title="${label}"${current}>${l.short}</a>`;
    }).join("")
    + `</nav>`;

  const values = new Map(strings);
  values.set("page.lang", locale.tag);
  values.set("page.og_locale", locale.og);
  values.set("page.home", home);
  values.set("page.canonical", `${ORIGIN}${home}`);
  values.set("page.alternates", alternates);
  values.set("page.lang_switcher", switcher);
  values.set("page.css", css.path);
  values.set("page.js", js.path);
  values.set("page.board_rows", boardRows(s("board.you")));
  // Sem link da loja ainda, os botões levam ao fim da página e dizem "em breve".
  values.set("cta.href", store ?? `${home}#fim`);
  values.set("cta.short", s(store ? "cta.short_live" : "cta.short_soon"));
  values.set("cta.over", s(store ? "cta.over_live" : "cta.over_soon"));
  values.set("page.store", storeButton(store, values.get("cta.over"), home, locale.tag));
  values.set("cta.note", s(store ? "cta.note_live" : "cta.note_soon"));

  const outDir = join(DIST, locale.dir);
  mkdirSync(outDir, { recursive: true });
  writeFileSync(join(outDir, "index.html"), render(indexTemplate, values, `${locale.tag}/index.html`));
  writeFileSync(join(outDir, "404.html"), render(notFoundTemplate, values, `${locale.tag}/404.html`));
}

copyFileSync(join(ROOT, "src/_headers"), join(DIST, "_headers"));
writeFileSync(join(DIST, css.path), css.body);
writeFileSync(join(DIST, js.path), js.body);
copyFileSync(FAVICON, join(DIST, "assets/logn-symbol-accent.svg"));
mkdirSync(join(DIST, "assets/badges"), { recursive: true });
for (const { tag } of LOCALES) {
  copyFileSync(join(ROOT, "src", badgePath(tag)), join(DIST, badgePath(tag)));
}
for (const font of FONTS) copyFileSync(join(FONTS_DIR, font), join(DIST, "fonts", font));

writeFileSync(join(DIST, "robots.txt"), `User-agent: *\nAllow: /\nSitemap: ${ORIGIN}/sitemap.xml\n`);
writeFileSync(join(DIST, "sitemap.xml"), [
  `<?xml version="1.0" encoding="UTF-8"?>`,
  `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">`,
  ...LOCALES.map((l) => `  <url><loc>${ORIGIN}/${l.dir}</loc>`
    + LOCALES.map((a) => `<xhtml:link rel="alternate" hreflang="${a.tag}" href="${ORIGIN}/${a.dir}"/>`).join("")
    + `</url>`),
  `</urlset>`,
  ``,
].join("\n"));

console.log(`landing: ${LOCALES.map((l) => l.tag).join(", ")} → dist/ (${store ? "App Store no ar" : "em breve"})`);
