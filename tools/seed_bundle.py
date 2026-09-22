#!/usr/bin/env python3
"""Empacota a trilha atual para dentro do app, para a primeira abertura sem rede.

Sem isto, instalação nova e offline não tem conteúdo nenhum — nem para quem tem conta,
porque o retrato do `crux_kv` ainda não existe. O app antes preenchia esse buraco com
uma trilha de mock: quatro nós inventados que o jogador não tinha como distinguir dos
reais, e cujos `node_id` não estão no banco, então responder ali gerava evento de sync
que o servidor não sabia creditar.

A fonte é a própria API, não o Postgres: assim o formato do arquivo é exatamente o que
o Core já sabe desserializar, e não existe um terceiro formato para manter em dia.

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
VERSION = 1

BASE = os.environ.get("SEED_BUNDLE_BASE_URL", "http://localhost:8080")
DESTINO = os.environ.get(
    "SEED_BUNDLE_OUT",
    "ios/LogNiOS/LogNiOS/Resources/trail-seed.json",
)


def buscar(caminho: str):
    url = f"{BASE}{caminho}"
    try:
        with urllib.request.urlopen(url, timeout=10) as r:
            if r.status != 200:
                sys.exit(f"{url} respondeu {r.status}")
            return json.loads(r.read())
    except urllib.error.URLError as e:
        sys.exit(
            f"não consegui falar com {url}: {e}\n"
            "o backend precisa estar de pé — `just run-backend` numa outra aba"
        )


def main() -> None:
    nodes = buscar("/api/v1/nodes")
    challenges = buscar("/api/v1/challenges")

    if not nodes:
        sys.exit("a API devolveu zero nós; não vou empacotar uma trilha vazia")

    semente = {
        "version": VERSION,
        "generated_at": datetime.datetime.now(datetime.timezone.utc)
        .replace(microsecond=0)
        .isoformat(),
        "nodes": nodes,
        "challenges": challenges,
    }

    os.makedirs(os.path.dirname(DESTINO), exist_ok=True)
    with open(DESTINO, "w", encoding="utf-8") as f:
        json.dump(semente, f, ensure_ascii=False, indent=2)
        f.write("\n")

    por_no: dict[str, int] = {}
    for c in challenges:
        por_no[c["node_id"]] = por_no.get(c["node_id"], 0) + 1
    vazios = sum(1 for n in nodes if n["id"] not in por_no)

    print(f"{DESTINO}")
    print(f"  versão {VERSION} · {len(nodes)} nós · {len(challenges)} desafios")
    if vazios:
        print(f"  {vazios} nó(s) ainda sem desafio")


if __name__ == "__main__":
    main()
