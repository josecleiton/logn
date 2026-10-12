#!/usr/bin/env python3
"""Empacota a trilha atual para dentro do app, para a primeira abertura sem rede.

Sem isto, instalação nova e offline não tem conteúdo nenhum — nem para quem tem conta,
porque o retrato do `crux_kv` ainda não existe. O app antes preenchia esse buraco com
uma trilha de mock: quatro nós inventados que o jogador não tinha como distinguir dos
reais, e cujos `node_id` não estão no banco, então responder ali gerava evento de sync
que o servidor não sabia creditar.

A fonte é a própria API, não o Postgres: assim o formato do arquivo é exatamente o que
o Core já sabe desserializar, e não existe um terceiro formato para manter em dia.

A semente leva uma trilha por língua, cada uma como a API serve com `?lang=`. Língua sem
nada publicado fica de fora, e o app cai em português. Com `--release`, as três línguas
têm de trazer os mesmos nós e desafios: build de loja não sai com trilha menor em uma
delas.

Rode de novo depois de cada migração que mexa em conteúdo — é o que `just seed-bundle`
faz. Nada aqui é secreto: o mesmo conteúdo já sai pelos endpoints públicos.
"""

import datetime
import json
import os
import sys
import urllib.error
import urllib.request

# Sobe quando o formato muda de um jeito que um app antigo não consegue ler. Tem de
# acompanhar TRAIL_SEED_VERSION em shared_core/src/domain.rs. Foi de 2 para 3 quando os
# cartões de origem passaram a viajar dentro da semente.
VERSION = 3

LANGUAGES = ["pt-BR", "en", "es"]

BASE = os.environ.get("SEED_BUNDLE_BASE_URL", "http://localhost:8080")
OUTPUT_PATH = os.environ.get(
    "SEED_BUNDLE_OUT",
    "ios/LogNiOS/LogNiOS/Resources/trail-seed.json",
)
# A Cloudflare na frente da produção barra o User-Agent padrão do urllib com 403.
USER_AGENT = "logn-seed-bundle/1"


def fetch(path: str, lang: str):
    url = f"{BASE}{path}?lang={lang}"
    try:
        request = urllib.request.Request(url, headers={"User-Agent": USER_AGENT})
        with urllib.request.urlopen(request, timeout=10) as r:
            if r.status != 200:
                sys.exit(f"{url} respondeu {r.status}")
            served = r.headers.get("Content-Language", "")
            if served != lang:
                sys.exit(f"{url} respondeu em {served or '(sem Content-Language)'}, não em {lang}")
            return json.loads(r.read())
    except urllib.error.URLError as e:
        sys.exit(
            f"não consegui falar com {url}: {e}\n"
            "o backend precisa estar de pé — `just run-backend` numa outra aba"
        )


def ids(items) -> set[str]:
    return {i["id"] for i in items}


def main() -> None:
    release = "--release" in sys.argv[1:]

    by_lang = {}
    for lang in LANGUAGES:
        nodes_response = fetch("/api/v1/nodes", lang)
        nodes = nodes_response["nodes"]
        origins = nodes_response.get("origins", [])
        challenges = fetch("/api/v1/challenges", lang)
        if nodes:
            by_lang[lang] = {"nodes": nodes, "challenges": challenges, "origins": origins}

    if "pt-BR" not in by_lang:
        sys.exit("a API devolveu zero nós em português; não vou empacotar uma trilha vazia")

    base = by_lang["pt-BR"]
    mismatched = [
        l for l in LANGUAGES
        if l not in by_lang
        or ids(by_lang[l]["nodes"]) != ids(base["nodes"])
        or ids(by_lang[l]["challenges"]) != ids(base["challenges"])
    ]
    if release and mismatched:
        sys.exit(
            f"--release: {', '.join(mismatched)} não traz(em) a mesma trilha que o português; "
            "termine a tradução ou gere sem --release"
        )

    seed = {
        "version": VERSION,
        "generated_at": datetime.datetime.now(datetime.timezone.utc)
        .replace(microsecond=0)
        .isoformat(),
        "locales": by_lang,
    }

    os.makedirs(os.path.dirname(OUTPUT_PATH), exist_ok=True)
    with open(OUTPUT_PATH, "w", encoding="utf-8") as f:
        json.dump(seed, f, ensure_ascii=False, indent=2)
        f.write("\n")

    print(f"{OUTPUT_PATH}")
    print(f"  versão {VERSION}")
    for lang in LANGUAGES:
        if lang not in by_lang:
            print(f"  {lang}: nada publicado; o app cai em português")
            continue
        trail = by_lang[lang]
        with_challenges = {c["node_id"] for c in trail["challenges"]}
        empty = sum(1 for n in trail["nodes"] if n["id"] not in with_challenges)
        line = f"  {lang}: {len(trail['nodes'])} nós · {len(trail['challenges'])} desafios"
        if empty:
            line += f" · {empty} nó(s) ainda sem desafio"
        print(line)


if __name__ == "__main__":
    main()
