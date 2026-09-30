# Terraform gerencia só o container do secret (nome, replicação). O valor nunca passa
# por aqui — nem em variável, nem em state. Sobe direto com:
#   printf '%s' "o-valor" | gcloud secrets versions add <nome> --data-file=-
# Isso vale pros três que já existem (import) e pro novo de verificação de origem.

resource "google_secret_manager_secret" "db_url" {
  secret_id = "logn-db-url"

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret" "db_migrator_url" {
  secret_id = "logn-db-migrator-url"

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret" "smtp_pass" {
  secret_id = "logn-smtp-pass"

  replication {
    auto {}
  }
}

# As chaves que o servidor gera e que ninguém mais lê, num JSON só (ADR 0019): um
# segredo por chave passava do free tier do Secret Manager, que cobre 6 versões ativas.
#   { "jwt_secret": "…", "track_key_secret": "…", "github_client_secret": "…" }
# - jwt_secret: trocar derruba todas as sessões.
# - track_key_secret: 32 bytes em base64; trocar tranca as trilhas compradas (ADR 0013).
#   É rotação com migração de dados, não troca de variável.
# - github_client_secret: só junto de github_client_id no tfvars; um sem o outro, o
#   servidor não sobe.
# Versão nova parte da anterior, trocando só o campo que muda, nunca à mão do zero:
#   gcloud secrets versions access latest --secret=logn-server-keys \
#     | jq --arg v "$NOVO" '.github_client_secret = $v' \
#     | gcloud secrets versions add logn-server-keys --data-file=-
# Substituiu logn-jwt-secret e logn-track-key-secret, um segredo por chave (ADR 0019).
resource "google_secret_manager_secret" "server_keys" {
  secret_id = "logn-server-keys"

  replication {
    auto {}
  }
}

# Criado sempre, mesmo com a verificação desligada (enable_origin_verification=false) —
# assim o container existe pra você já subir o valor com gcloud antes de ligar a
# checagem no Cloud Run, sem depender de ordem entre dois applies.
resource "google_secret_manager_secret" "origin_shared_secret" {
  secret_id = "logn-origin-shared-secret"

  replication {
    auto {}
  }
}

# A chave `.p8` do Sign in with Apple (ADR 0017), em PEM. É com ela que o servidor
# revoga o acesso na exclusão da conta. Criado sempre, como o de origem, e referenciado
# pelo Cloud Run só com enable_apple_signin=true, depois de subir o valor:
#   gcloud secrets versions add logn-apple-signin-key --data-file=AuthKey_XXXXXXXXXX.p8
resource "google_secret_manager_secret" "apple_signin_key" {
  secret_id = "logn-apple-signin-key"

  replication {
    auto {}
  }
}

# A conta com que o serviço roda lê cada segredo que o Cloud Run referencia, um por um,
# e nenhum outro. Ela tinha roles/secretmanager.secretAccessor no projeto: todo segredo
# criado ali, de qualquer coisa, ela passaria a ler sem ninguém decidir. Segredo novo
# que o serviço use entra neste mapa (AGENTS.md, regra 9).
locals {
  runtime_secrets = {
    db_url               = google_secret_manager_secret.db_url.id
    db_migrator_url      = google_secret_manager_secret.db_migrator_url.id
    server_keys          = google_secret_manager_secret.server_keys.id
    smtp_pass            = google_secret_manager_secret.smtp_pass.id
    origin_shared_secret = google_secret_manager_secret.origin_shared_secret.id
    apple_signin_key     = google_secret_manager_secret.apple_signin_key.id
  }
}

resource "google_secret_manager_secret_iam_member" "runtime_reads" {
  for_each = local.runtime_secrets

  secret_id = each.value
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${var.service_account_email}"
}

# Os papéis amplos que a conta de runtime tinha no projeto, dados à mão, fora deste
# Terraform. O backend não usa Cloud Storage nem Firebase, e o segredo passa a ser lido
# pelo acesso acima. Tirá-los só depois de o acesso por segredo existir: o Cloud Run lê
# os segredos a cada instância que sobe, e um intervalo sem acesso derrubaria a próxima.
resource "google_project_iam_member_remove" "runtime_broad_roles" {
  for_each = toset([
    "roles/secretmanager.secretAccessor",
    "roles/storage.editor",
    "roles/firebase.editor",
  ])

  project = var.project_id
  role    = each.key
  member  = "serviceAccount:${var.service_account_email}"

  depends_on = [google_secret_manager_secret_iam_member.runtime_reads]
}
