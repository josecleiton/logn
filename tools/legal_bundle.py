#!/usr/bin/env python3
"""Empacota termos e política, no modo do app, para ler sem rede.

A tela de documento legal do iOS carrega `/legal/<kind>?embed=1` do servidor. Sem rede,
cai na cópia que este script deixa no bundle: as seis combinações de documento e
língua, já montadas pelo próprio servidor, mais um manifesto com versão e data de cada
uma — o cabeçalho nativo mostra as duas, e offline não há cabeçalho HTTP para lê-las.

A fonte é a própria API, como no `seed_bundle.py`: a página que o app mostra offline é
byte a byte a que ele mostraria online.

Recusa documento em rascunho, a não ser com LEGAL_BUNDLE_ALLOW_DRAFT=1: um build de
loja não pode levar [A CONFIRMAR] para dentro do app.
"""

import json
import os
import sys
import urllib.error
import urllib.request

BASE = os.environ.get("LEGAL_BUNDLE_BASE_URL", "http://localhost:8080")
DESTINO = os.environ.get("LEGAL_BUNDLE_OUT", "ios/LogNiOS/LogNiOS/Resources/Legal")
ALLOW_DRAFT = os.environ.get("LEGAL_BUNDLE_ALLOW_DRAFT") == "1"

KINDS = ["terms", "privacy"]
LOCALES = ["pt-BR", "en", "es"]


def buscar(kind: str, locale: str):
    url = f"{BASE}/legal/{kind}?embed=1&lang={locale}"
    try:
        with urllib.request.urlopen(url, timeout=10) as r:
            if r.status != 200:
                sys.exit(f"{url} respondeu {r.status}")
            servida = r.headers.get("Content-Language")
            if servida != locale:
                sys.exit(f"{url} devolveu {servida}, não {locale}: falta a tradução no banco")
            return r.read(), r.headers
    except urllib.error.HTTPError as e:
        sys.exit(f"{url} respondeu {e.code}")
    except urllib.error.URLError as e:
        sys.exit(
            f"não consegui falar com {url}: {e}\n"
            "o backend precisa estar de pé — `just run-backend` numa outra aba"
        )


def main() -> None:
    os.makedirs(DESTINO, exist_ok=True)
    manifesto = {}
    for kind in KINDS:
        for locale in LOCALES:
            corpo, headers = buscar(kind, locale)
            rascunho = headers.get("X-LogN-Legal-Draft") == "1"
            if rascunho and not ALLOW_DRAFT:
                sys.exit(
                    f"{kind}.{locale} ainda é rascunho. Publique o texto final, "
                    "ou rode com LEGAL_BUNDLE_ALLOW_DRAFT=1 num build que não vai para a loja."
                )
            nome = f"legal-{kind}.{locale}.html"
            with open(os.path.join(DESTINO, nome), "wb") as f:
                f.write(corpo)
            manifesto.setdefault(kind, {})[locale] = {
                "version": int(headers["X-LogN-Legal-Version"]),
                "effective_at": headers["X-LogN-Legal-Effective"],
                "draft": rascunho,
            }
            print(f"{nome}: v{manifesto[kind][locale]['version']}"
                  f" de {manifesto[kind][locale]['effective_at']}"
                  f"{' (rascunho)' if rascunho else ''}")

    with open(os.path.join(DESTINO, "legal-manifest.json"), "w", encoding="utf-8") as f:
        json.dump(manifesto, f, ensure_ascii=False, indent=2)
        f.write("\n")
    print(f"manifesto em {DESTINO}/legal-manifest.json")


if __name__ == "__main__":
    main()
