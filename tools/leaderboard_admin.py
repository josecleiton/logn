#!/usr/bin/env python3
"""Modera o placar e atende a objeção a ele (docs/specs/logn_placar_spec.md, seção 8).

    python3 tools/leaderboard_admin.py <anonymize|hide|unhide> <conta> <motivo>

anonymize  volta o apelido ao número, queima a chance de escolher outro e bloqueia o
           apelido para todo mundo. É para apelido ofensivo.
hide       tira a conta do placar. É a objeção que chega por e-mail.
unhide     devolve a conta ao placar, se a pessoa pedir.

A conta sai do jeito que você a tem na mão:
    um UUID                 o id da conta
    #4821                   o número de "jogador #4821" (o # é obrigatório: o apelido
                            pode ser só de dígitos, e "4821" é o apelido)
    algo com @              o e-mail de quem escreveu (não vai para o log)
    o resto                 o apelido

O motivo fica gravado em leaderboard_actions. Escreva o que foi visto ("apelido
ofensivo no placar, 2026-10-03"), sem copiar e-mail ou dado pessoal.

Quem roda precisa do mesmo acesso de `just revoke` (ver tools/license_admin.py).
"""

import json
import re
import sys

from license_admin import identity_token, post_json, service_env

UUID = re.compile(r"^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$")


def target(who: str) -> dict:
    who = who.strip()
    if UUID.match(who):
        return {"user_id": who.lower()}
    if re.fullmatch(r"#\d+", who):
        return {"anon_number": int(who[1:])}
    if "@" in who:
        return {"email": who}
    return {"nickname": who}


def main() -> None:
    args = sys.argv[1:]
    if len(args) != 3 or args[0] not in ("anonymize", "hide", "unhide"):
        sys.exit(__doc__)
    body = {"action": args[0], "reason": args[2], **target(args[1])}

    env = service_env()
    audience = env.get("CLOUD_SCHEDULER_AUDIENCE", "")
    admin = env.get("ADMIN_SERVICE_ACCOUNT", "")
    if not audience or not admin:
        sys.exit("o serviço não tem CLOUD_SCHEDULER_AUDIENCE ou ADMIN_SERVICE_ACCOUNT: rode o terraform apply antes")

    token = identity_token(admin, audience)
    result = post_json(f"{audience}/api/v1/internal/leaderboard/actions", body, token)
    print(json.dumps(result, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
