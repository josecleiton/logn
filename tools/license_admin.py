#!/usr/bin/env python3
"""Revoga à mão a licença de uma trilha paga e responde à contestação (ADR 0021).

    python3 tools/license_admin.py revoke <user_id> <track_id> <redistribution|account_sharing> <evidência>
    python3 tools/license_admin.py appeal <user_id> <track_id> <received|review|accepted|rejected> <evidência>

A contestação segue a seção 10.5 dos termos:
    received   redistribuição: confirma que recebemos; a trilha segue fechada.
    review     compartilhamento de conta: a licença volta enquanto analisamos.
    accepted   a licença volta, com o progresso.
    rejected   a revogação fica (no compartilhamento, só depois de review).

Quem roda precisa de `roles/iam.serviceAccountOpenIdTokenCreator` sobre a conta
logn-admin (`admin_members` no Terraform). O token de identidade é emitido em nome dela
pelo IAM Credentials, com a sua conta Google do gcloud; nenhuma chave sai do Google. A audiência e a conta saem do próprio serviço
no Cloud Run, as mesmas que o backend confere, e o pedido vai direto para a URL
.run.app, sem passar pela Cloudflare.

A evidência fica gravada no banco com a revogação. Escreva o que foi visto e onde está
("dois aparelhos ativos na mesma hora, 2026-10-02"), sem copiar e-mail ou dado pessoal.
"""

import json
import os
import subprocess
import sys
import urllib.error
import urllib.request

SERVICE = os.environ.get("LOGN_SERVICE", "logn")
REGION = os.environ.get("LOGN_REGION", "us-east1")
USER_AGENT = "logn-license-admin/1"


def gcloud(*args: str) -> str:
    try:
        return subprocess.run(["gcloud", *args], check=True, capture_output=True, text=True).stdout.strip()
    except subprocess.CalledProcessError as e:
        sys.exit(f"gcloud {args[0]} falhou:\n{e.stderr.strip()}")


def service_env() -> dict:
    raw = gcloud("run", "services", "describe", SERVICE, "--region", REGION, "--format", "json")
    containers = json.loads(raw)["spec"]["template"]["spec"]["containers"]
    return {e["name"]: e.get("value", "") for e in containers[0].get("env", []) if "value" in e}


def post_json(url: str, body: dict, token: str) -> dict:
    req = urllib.request.Request(url, data=json.dumps(body).encode(), method="POST", headers={
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json",
        "User-Agent": USER_AGENT,
    })
    try:
        with urllib.request.urlopen(req, timeout=30) as r:
            return json.load(r)
    except urllib.error.HTTPError as e:
        sys.exit(f"{url} respondeu {e.code}: {e.read().decode(errors='replace').strip()}")
    except urllib.error.URLError as e:
        sys.exit(f"não consegui falar com {url}: {e}")


def identity_token(admin: str, audience: str) -> str:
    """O token de identidade da logn-admin, pedido direto ao IAM Credentials.

    O `gcloud auth print-identity-token --impersonate-service-account` pede antes um token
    de acesso da conta, o que exige o TokenCreator inteiro. Aqui a sua conta pede só o
    token de identidade, que é o que o `roles/iam.serviceAccountOpenIdTokenCreator` dá.
    """
    own = gcloud("auth", "print-access-token")
    url = f"https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/{admin}:generateIdToken"
    token = post_json(url, {"audience": audience, "includeEmail": True}, own).get("token", "")
    if not token:
        sys.exit("o IAM Credentials não devolveu token: você está em admin_members?")
    return token


def main() -> None:
    args = sys.argv[1:]
    if len(args) != 5 or args[0] not in ("revoke", "appeal"):
        sys.exit(__doc__)
    action, user_id, track_id = args[0], args[1], args[2]
    if action == "revoke":
        body = {"user_id": user_id, "track_id": track_id, "reason": args[3], "evidence": args[4]}
    else:
        body = {"user_id": user_id, "track_id": track_id, "outcome": args[3], "evidence": args[4]}

    env = service_env()
    audience = env.get("CLOUD_SCHEDULER_AUDIENCE", "")
    admin = env.get("ADMIN_SERVICE_ACCOUNT", "")
    if not audience or not admin:
        sys.exit("o serviço não tem CLOUD_SCHEDULER_AUDIENCE ou ADMIN_SERVICE_ACCOUNT: rode o terraform apply antes")

    token = identity_token(admin, audience)

    result = post_json(f"{audience}/api/v1/internal/licenses/{action}", body, token)
    print(json.dumps(result, ensure_ascii=False, indent=2))
    if not result.get("email_sent"):
        sys.exit("a mudança foi gravada, mas o aviso por e-mail NÃO saiu: avise a pessoa por contact@logn.sh")


if __name__ == "__main__":
    main()
