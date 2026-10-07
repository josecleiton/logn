#!/usr/bin/env python3
"""Sobe o .aab do `just android/release` ao teste interno do Google Play, e de lá à produção.

    python3 tools/play_upload.py check               # confere notas, versão e .aab; não sai pedido
    python3 tools/play_upload.py upload [completed|draft]
    python3 tools/play_upload.py promote             # o build do teste interno vai à produção

As notas da versão saem de `android/release-notes.txt`, no formato que o Play Console
aceita colar, uma tag por língua:

    <pt-BR>
    ...
    </pt-BR>

As três línguas do app (pt-BR, en-US, es-419) são obrigatórias, cada uma com até 500
caracteres. O nome da versão e o versionCode saem de `version.properties`; o .aab cujo
versionCode não bate com ele é recusado antes do commit da edição.

`completed` publica para os testadores na hora. `draft` deixa a versão como rascunho no
Console, para revisar e lançar à mão; é o único estado que o Play aceita enquanto o app
não tem nenhuma versão publicada.

Quem roda precisa de `roles/iam.serviceAccountTokenCreator` sobre a conta
logn-play-publisher (`play_publishers` no Terraform), e a conta precisa estar convidada
no Play Console com permissão de lançar em faixas de teste. O token de acesso é emitido
em nome dela pelo IAM Credentials, com a sua conta Google do gcloud; nenhuma chave sai
do Google.
"""

from __future__ import annotations

import json
import os
import re
import subprocess
import sys
import urllib.error
import urllib.request

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
PACKAGE = os.environ.get("PLAY_PACKAGE_NAME", "sh.logn.app")
AAB = os.path.join(ROOT, "android", "build", "store", "LogN.aab")
NOTES = os.path.join(ROOT, "android", "release-notes.txt")
VERSION = os.path.join(ROOT, "version.properties")
TRACK = "internal"
LANGUAGES = ["pt-BR", "en-US", "es-419"]
NOTE_LIMIT = 500
# O texto que o Console põe no lugar das notas: colado sem trocar, ia para os testadores.
PLACEHOLDER = "Enter or paste your release notes"
API = f"https://androidpublisher.googleapis.com/androidpublisher/v3/applications/{PACKAGE}"
UPLOAD_API = f"https://androidpublisher.googleapis.com/upload/androidpublisher/v3/applications/{PACKAGE}"
SCOPE = "https://www.googleapis.com/auth/androidpublisher"
USER_AGENT = "logn-play-upload/1"


def gcloud(*args: str) -> str:
    try:
        return subprocess.run(["gcloud", *args], check=True, capture_output=True, text=True).stdout.strip()
    except subprocess.CalledProcessError as e:
        sys.exit(f"gcloud {args[0]} falhou:\n{e.stderr.strip()}")


class HttpError(Exception):
    pass


def request(method: str, url: str, token: str, body: dict | None = None, data: bytes | None = None) -> dict:
    headers = {"Authorization": f"Bearer {token}", "User-Agent": USER_AGENT}
    if body is not None:
        data = json.dumps(body).encode()
        headers["Content-Type"] = "application/json"
    elif data is not None:
        headers["Content-Type"] = "application/octet-stream"
    req = urllib.request.Request(url, data=data, method=method, headers=headers)
    try:
        # O .aab tem dezenas de MB: o envio leva mais que os outros pedidos.
        with urllib.request.urlopen(req, timeout=600) as r:
            raw = r.read()
            return json.loads(raw) if raw else {}
    except urllib.error.HTTPError as e:
        raise HttpError(f"{method} {url} respondeu {e.code}: {e.read().decode(errors='replace').strip()}") from e
    except urllib.error.URLError as e:
        raise HttpError(f"não consegui falar com {url}: {e}") from e


def version() -> tuple[str, int]:
    props = {}
    with open(VERSION, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if line and not line.startswith("#") and "=" in line:
                key, value = line.split("=", 1)
                props[key.strip()] = value.strip()
    try:
        return props["name"], int(props["build"])
    except (KeyError, ValueError):
        sys.exit(f"{VERSION} sem name ou build numérico")


def release_notes() -> list[dict]:
    """As notas por língua, conferidas. Falta, sobra, vazio ou texto do Console param aqui."""
    if not os.path.exists(NOTES):
        sys.exit(f"falta {os.path.relpath(NOTES, ROOT)}, com uma tag por língua ({', '.join(LANGUAGES)})")
    with open(NOTES, encoding="utf-8") as f:
        text = f.read()
    found = {lang: body.strip() for lang, body in re.findall(r"<([A-Za-z]{2,3}(?:-[A-Za-z0-9]+)?)>(.*?)</\1>", text, re.S)}
    problems = []
    for lang in LANGUAGES:
        body = found.get(lang)
        if body is None:
            problems.append(f"{lang}: falta a tag")
        elif not body:
            problems.append(f"{lang}: vazia")
        elif PLACEHOLDER in body:
            problems.append(f"{lang}: ainda é o texto de exemplo do Console")
        elif len(body) > NOTE_LIMIT:
            problems.append(f"{lang}: {len(body)} caracteres, o Play aceita {NOTE_LIMIT}")
    extra = sorted(set(found) - set(LANGUAGES))
    if extra:
        problems.append(f"língua que o app não tem: {', '.join(extra)}")
    if problems:
        sys.exit("notas da versão recusadas:\n  " + "\n  ".join(problems))
    return [{"language": lang, "text": found[lang]} for lang in LANGUAGES]


def check() -> tuple[str, int, list[dict]]:
    name, build = version()
    notes = release_notes()
    if not os.path.exists(AAB):
        sys.exit(f"falta {os.path.relpath(AAB, ROOT)}: rode `just android/release` antes")
    return name, build, notes


def access_token() -> str:
    """O token de acesso da logn-play-publisher, pedido ao IAM Credentials com a sua conta."""
    account = os.environ.get("PLAY_PUBLISHER") or f"logn-play-publisher@{gcloud('config', 'get', 'project')}.iam.gserviceaccount.com"
    own = gcloud("auth", "print-access-token")
    url = f"https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/{account}:generateAccessToken"
    try:
        token = request("POST", url, own, body={"scope": [SCOPE], "lifetime": "1800s"}).get("accessToken", "")
    except HttpError as e:
        sys.exit(f"{e}\nVocê está em play_publishers no Terraform, e o apply já rodou?")
    if not token:
        sys.exit("o IAM Credentials não devolveu token")
    return token


def in_edit(token: str, steps) -> None:
    """Roda `steps(edit)` numa edição nova e faz o commit; se algo falha, a edição some."""
    edit = request("POST", f"{API}/edits", token, body={})["id"]
    try:
        steps(edit)
        request("POST", f"{API}/edits/{edit}:commit", token)
    except HttpError as e:
        # A edição aberta prende o app: outra edição só depois de esta expirar.
        try:
            request("DELETE", f"{API}/edits/{edit}", token)
        except HttpError:
            pass
        hint = "\nApp sem versão publicada aceita só rascunho: rode com status=draft." if "draft" in str(e).lower() else ""
        sys.exit(f"{e}{hint}")


def upload(status: str) -> None:
    name, build, notes = check()
    token = access_token()

    def steps(edit: str) -> None:
        print(f"Enviando {os.path.relpath(AAB, ROOT)} ({os.path.getsize(AAB) // 1_000_000} MB)…")
        with open(AAB, "rb") as f:
            bundle = request("POST", f"{UPLOAD_API}/edits/{edit}/bundles?uploadType=media", token, data=f.read())
        code = int(bundle.get("versionCode", 0))
        if code != build:
            raise HttpError(f"o .aab tem versionCode {code}, e version.properties diz {build}: rode `just android/release` de novo")
        release = {"name": f"{name} ({build})", "versionCodes": [str(build)], "status": status, "releaseNotes": notes}
        request("PUT", f"{API}/edits/{edit}/tracks/{TRACK}", token, body={"track": TRACK, "releases": [release]})

    in_edit(token, steps)
    state = "publicada para os testadores" if status == "completed" else "em rascunho no Console"
    print(f"Pronto: {name} ({build}) no teste interno, {state}, com notas em {', '.join(LANGUAGES)}.")


def promote() -> None:
    """Leva a produção a versão do teste interno que tem o build de version.properties.

    Nada sobe de novo: o .aab é o mesmo que os testadores receberam, com as mesmas notas.
    A produção passa pela revisão do Play antes de chegar à loja.
    """
    name, build = version()
    token = access_token()

    def steps(edit: str) -> None:
        internal = request("GET", f"{API}/edits/{edit}/tracks/{TRACK}", token)
        found = [r for r in internal.get("releases", []) if str(build) in r.get("versionCodes", [])]
        if not found:
            raise HttpError(f"o build {build} não está no teste interno: rode `just android/play-internal` antes")
        tested = found[0]
        release = {
            "name": tested.get("name", f"{name} ({build})"),
            "versionCodes": [str(build)],
            "status": "completed",
            "releaseNotes": tested.get("releaseNotes", []),
        }
        request("PUT", f"{API}/edits/{edit}/tracks/production", token, body={"track": "production", "releases": [release]})

    in_edit(token, steps)
    print(f"Pronto: {name} ({build}) enviada à produção; chega à loja depois da revisão do Play.")


def main() -> None:
    args = sys.argv[1:]
    if args == ["check"]:
        name, build, _ = check()
        print(f"ok: {name} ({build}), {os.path.relpath(AAB, ROOT)} e notas em {', '.join(LANGUAGES)}")
    elif args and args[0] == "upload" and len(args) <= 2:
        status = args[1] if len(args) == 2 else "completed"
        if status not in ("completed", "draft"):
            sys.exit("status é completed ou draft")
        upload(status)
    elif args == ["promote"]:
        promote()
    else:
        sys.exit(__doc__)


if __name__ == "__main__":
    main()
