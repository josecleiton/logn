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

# Cifra as chaves de conteúdo das trilhas pagas em `track_keys` (ADR 0012): 32 bytes em
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
