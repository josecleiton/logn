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
OUTPUT_DIR = os.environ.get("LEGAL_BUNDLE_OUT", "ios/LogNiOS/LogNiOS/Resources/Legal")
ALLOW_DRAFT = os.environ.get("LEGAL_BUNDLE_ALLOW_DRAFT") == "1"
# A Cloudflare na frente da produção barra o User-Agent padrão do urllib com 403.
USER_AGENT = "logn-legal-bundle/1"

KINDS = ["terms", "privacy"]
LOCALES = ["pt-BR", "en", "es"]


def fetch(kind: str, locale: str):
    url = f"{BASE}/legal/{kind}?embed=1&lang={locale}"
    try:
        request = urllib.request.Request(url, headers={"User-Agent": USER_AGENT})
        with urllib.request.urlopen(request, timeout=10) as r:
            if r.status != 200:
                sys.exit(f"{url} respondeu {r.status}")
            served = r.headers.get("Content-Language")
            if served != locale:
                sys.exit(f"{url} devolveu {served}, não {locale}: falta a tradução no banco")
            return r.read(), r.headers
    except urllib.error.HTTPError as e:
        sys.exit(f"{url} respondeu {e.code}")
    except urllib.error.URLError as e:
        sys.exit(
            f"não consegui falar com {url}: {e}\n"
            "o backend precisa estar de pé — `just run-backend` numa outra aba"
        )


def main() -> None:
    os.makedirs(OUTPUT_DIR, exist_ok=True)
    manifest = {}
    for kind in KINDS:
        for locale in LOCALES:
            body, headers = fetch(kind, locale)
            is_draft = headers.get("X-LogN-Legal-Draft") == "1"
            if is_draft and not ALLOW_DRAFT:
                sys.exit(
                    f"{kind}.{locale} ainda é rascunho. Publique o texto final, "
                    "ou rode com LEGAL_BUNDLE_ALLOW_DRAFT=1 num build que não vai para a loja."
                )
            name = f"legal-{kind}.{locale}.html"
            with open(os.path.join(OUTPUT_DIR, name), "wb") as f:
                f.write(body)
            manifest.setdefault(kind, {})[locale] = {
                "version": int(headers["X-LogN-Legal-Version"]),
                "effective_at": headers["X-LogN-Legal-Effective"],
                "draft": is_draft,
            }
            print(f"{name}: v{manifest[kind][locale]['version']}"
                  f" de {manifest[kind][locale]['effective_at']}"
                  f"{' (rascunho)' if is_draft else ''}")

    with open(os.path.join(OUTPUT_DIR, "legal-manifest.json"), "w", encoding="utf-8") as f:
        json.dump(manifest, f, ensure_ascii=False, indent=2)
        f.write("\n")
    print(f"manifesto em {OUTPUT_DIR}/legal-manifest.json")


if __name__ == "__main__":
    main()
