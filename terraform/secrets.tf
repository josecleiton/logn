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

resource "google_secret_manager_secret" "jwt_secret" {
  secret_id = "logn-jwt-secret"

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

# Cifra as chaves de conteúdo das trilhas pagas em `track_keys` (ADR 0013): 32 bytes em
# base64. O servidor não sobe no Cloud Run sem ele. Trocar o valor invalida as chaves
# guardadas — é rotação com migração de dados, não troca de variável.
#   openssl rand -base64 32 | tr -d '\n' | gcloud secrets versions add logn-track-key-secret --data-file=-
resource "google_secret_manager_secret" "track_key_secret" {
  secret_id = "logn-track-key-secret"

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

# A conta com que o serviço roda lê cada segredo que o Cloud Run referencia, um por um,
# e nenhum outro. Ela tinha roles/secretmanager.secretAccessor no projeto: todo segredo
# criado ali, de qualquer coisa, ela passaria a ler sem ninguém decidir. Segredo novo
# que o serviço use entra neste mapa (AGENTS.md, regra 9).
locals {
  runtime_secrets = {
    db_url               = google_secret_manager_secret.db_url.id
    db_migrator_url      = google_secret_manager_secret.db_migrator_url.id
    jwt_secret           = google_secret_manager_secret.jwt_secret.id
    smtp_pass            = google_secret_manager_secret.smtp_pass.id
    track_key_secret     = google_secret_manager_secret.track_key_secret.id
    origin_shared_secret = google_secret_manager_secret.origin_shared_secret.id
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
