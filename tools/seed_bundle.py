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
# acompanhar TRAIL_SEED_VERSION em shared_core/src/domain.rs.
VERSION = 2

LINGUAS = ["pt-BR", "en", "es"]

BASE = os.environ.get("SEED_BUNDLE_BASE_URL", "http://localhost:8080")
DESTINO = os.environ.get(
    "SEED_BUNDLE_OUT",
    "ios/LogNiOS/LogNiOS/Resources/trail-seed.json",
)


def buscar(caminho: str, lingua: str):
    url = f"{BASE}{caminho}?lang={lingua}"
    try:
        with urllib.request.urlopen(url, timeout=10) as r:
            if r.status != 200:
                sys.exit(f"{url} respondeu {r.status}")
            servida = r.headers.get("Content-Language", "")
            if servida != lingua:
                sys.exit(f"{url} respondeu em {servida or '(sem Content-Language)'}, não em {lingua}")
            return json.loads(r.read())
    except urllib.error.URLError as e:
        sys.exit(
            f"não consegui falar com {url}: {e}\n"
            "o backend precisa estar de pé — `just run-backend` numa outra aba"
        )


def ids(itens) -> set[str]:
    return {i["id"] for i in itens}


def main() -> None:
    release = "--release" in sys.argv[1:]

    linguas = {}
    for lingua in LINGUAS:
        nodes = buscar("/api/v1/nodes", lingua)
        challenges = buscar("/api/v1/challenges", lingua)
        if nodes:
            linguas[lingua] = {"nodes": nodes, "challenges": challenges}

    if "pt-BR" not in linguas:
        sys.exit("a API devolveu zero nós em português; não vou empacotar uma trilha vazia")

    base = linguas["pt-BR"]
    diferentes = [
        l for l in LINGUAS
        if l not in linguas
        or ids(linguas[l]["nodes"]) != ids(base["nodes"])
        or ids(linguas[l]["challenges"]) != ids(base["challenges"])
    ]
    if release and diferentes:
        sys.exit(
            f"--release: {', '.join(diferentes)} não traz(em) a mesma trilha que o português; "
            "termine a tradução ou gere sem --release"
        )

    semente = {
        "version": VERSION,
        "generated_at": datetime.datetime.now(datetime.timezone.utc)
        .replace(microsecond=0)
        .isoformat(),
        "locales": linguas,
    }

    os.makedirs(os.path.dirname(DESTINO), exist_ok=True)
    with open(DESTINO, "w", encoding="utf-8") as f:
        json.dump(semente, f, ensure_ascii=False, indent=2)
        f.write("\n")

    print(f"{DESTINO}")
    print(f"  versão {VERSION}")
    for lingua in LINGUAS:
        if lingua not in linguas:
            print(f"  {lingua}: nada publicado; o app cai em português")
            continue
        trilha = linguas[lingua]
        com_desafio = {c["node_id"] for c in trilha["challenges"]}
        vazios = sum(1 for n in trilha["nodes"] if n["id"] not in com_desafio)
        linha = f"  {lingua}: {len(trilha['nodes'])} nós · {len(trilha['challenges'])} desafios"
        if vazios:
            linha += f" · {vazios} nó(s) ainda sem desafio"
        print(linha)


if __name__ == "__main__":
    main()
