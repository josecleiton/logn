locals {
  # Alvo da rota interna de purga (Cloud Scheduler + a env que o backend valida).
  # Sempre a URL .run.app, mesmo com Cloudflare ligada: o Bot Fight Mode da zona não
  # aceita exceção no plano free e desafiaria o Scheduler. Direto no Cloud Run, a rota
  # fica isenta da verificação de origem (originExempt em main.go) e protegida só pelo
  # OIDC. Scheduler.tf usa o mesmo valor; os dois têm que bater ou o OIDC falha.
  scheduler_audience = var.service_audience_url

  # Com Cloudflare ligada, os CIDRs vêm do data source em cloudflare.tf (sempre
  # atualizados); senão, cai no que foi preenchido à mão em origin_trusted_cidrs — é o
  # caminho pra usar verificação de origem com outro proxy que não a Cloudflare.
  origin_cidrs = var.enable_cloudflare ? local.cloudflare_cidrs : var.origin_trusted_cidrs

  # Env vars fixos de verificação de origem só entram na lista quando ligada — assim
  # o plano fica idêntico ao estado atual enquanto enable_origin_verification=false.
  origin_env = var.enable_origin_verification ? [
    {
      name  = "ORIGIN_TRUSTED_CIDRS"
      value = local.origin_cidrs
    }
  ] : []
}

resource "google_cloud_run_v2_service" "logn" {
  name                = "logn"
  location            = var.region
  ingress             = "INGRESS_TRAFFIC_ALL"
  deletion_protection = true

  # O deploy de imagem continua fora do Terraform: `just deploy-backend` builda e
  # publica no Artifact Registry, e só então `terraform apply -var container_image=...`
  # atualiza o serviço para apontar pra ela. Terraform nunca builda a partir do
  # Dockerfile.
  template {
    service_account = var.service_account_email
    timeout         = "60s"

    scaling {
      max_instance_count = 1
    }

    max_instance_request_concurrency = 100

    containers {
      name  = "logn-api"
      image = var.container_image

      ports {
        name           = "http1"
        container_port = 8080
      }

      resources {
        limits = {
          cpu    = "1000m"
          memory = "256Mi"
        }
        cpu_idle          = true
        startup_cpu_boost = true
      }

      startup_probe {
        http_get {
          path = "/ready"
          port = 8080
        }
        initial_delay_seconds = 2
        period_seconds        = 15
        timeout_seconds       = 10
        failure_threshold     = 6
      }

      liveness_probe {
        http_get {
          path = "/health"
          port = 8080
        }
        period_seconds    = 10
        timeout_seconds   = 1
        failure_threshold = 3
      }

      env {
        name  = "SMTP_USER"
        value = var.smtp_user
      }
      env {
        name  = "SMTP_HOST"
        value = var.smtp_host
      }
      env {
        name  = "SMTP_PORT"
        value = var.smtp_port
      }
      env {
        name  = "SMTP_FROM"
        value = var.smtp_from
      }
      env {
        name  = "CLOUD_SCHEDULER_AUDIENCE"
        value = local.scheduler_audience
      }
      # Quem assina cada rota interna (ADR 0021). Audiência certa não basta: qualquer
      # conta de serviço consegue um token com ela. A purga aceita só a logn-scheduler;
      # as de licença, só a logn-admin (admin.tf).
      env {
        name  = "CLOUD_SCHEDULER_SERVICE_ACCOUNT"
        value = google_service_account.scheduler.email
      }
      env {
        name  = "ADMIN_SERVICE_ACCOUNT"
        value = google_service_account.admin.email
      }
      env {
        name  = "RUN_MIGRATIONS"
        value = "true"
      }
      # O bundle que as transações da App Store têm de trazer. Não é segredo; sem ele o
      # servidor não sobe no Cloud Run (store_config.go).
      env {
        name  = "APPLE_BUNDLE_ID"
        value = "sh.logn.app"
      }
      # A audiência do ID token do Google. Vazia, o login com Google fica desligado
      # (social_handlers.go); o resto do serviço não depende dela.
      env {
        name  = "GOOGLE_IOS_CLIENT_ID"
        value = var.google_ios_client_id
      }
      # O Client ID do OAuth App do GitHub (ADR 0019). Vazio, o GitHub fica desligado; o
      # secret vem em SERVER_KEYS, e os dois só valem juntos: um sem o outro, o servidor
      # não sobe.
      env {
        name  = "GITHUB_CLIENT_ID"
        value = var.github_client_id
      }

      env {
        name = "DATABASE_URL"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.db_url.secret_id
            version = "latest"
          }
        }
      }
      env {
        name = "DATABASE_MIGRATION_URL"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.db_migrator_url.secret_id
            version = "latest"
          }
        }
      }
      # JWT, track key e o secret do GitHub, num JSON só (ADR 0019). Substitui JWT_SECRET
      # e TRACK_KEY_SECRET, que eram um segredo cada.
      env {
        name = "SERVER_KEYS"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.server_keys.secret_id
            version = "latest"
          }
        }
      }
      env {
        name = "SMTP_PASS"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.smtp_pass.secret_id
            version = "latest"
          }
        }
      }

      dynamic "env" {
        for_each = local.origin_env
        content {
          name  = env.value.name
          value = env.value.value
        }
      }

      # Lista de espera do iPhone (ADR 0022). Os dois domínios públicos: a landing, para
      # onde as rotas respondem com 303, e a API, para os links do e-mail. Desligada, as
      # rotas não existem; uma variável sem a outra, o servidor não sobe.
      dynamic "env" {
        for_each = var.enable_waitlist && var.domain_name != "" ? {
          WAITLIST_LANDING_ORIGIN = "https://${var.domain_name}"
          WAITLIST_API_ORIGIN     = "https://${var.api_subdomain}.${var.domain_name}"
        } : {}
        content {
          name  = env.key
          value = env.value
        }
      }

      # Sign in with Apple (ADR 0017). Desligado, o servidor responde provider_disabled
      # para a Apple; as três vêm juntas, ou o servidor não sobe.
      dynamic "env" {
        for_each = var.enable_apple_signin ? {
          APPLE_SIGNIN_TEAM_ID = var.apple_signin_team_id
          APPLE_SIGNIN_KEY_ID  = var.apple_signin_key_id
        } : {}
        content {
          name  = env.key
          value = env.value
        }
      }

      dynamic "env" {
        for_each = var.enable_apple_signin ? [1] : []
        content {
          name = "APPLE_SIGNIN_PRIVATE_KEY"
          value_source {
            secret_key_ref {
              secret  = google_secret_manager_secret.apple_signin_key.secret_id
              version = "latest"
            }
          }
        }
      }

      dynamic "env" {
        for_each = var.enable_origin_verification ? [1] : []
        content {
          name = "ORIGIN_SHARED_SECRET"
          value_source {
            secret_key_ref {
              secret  = google_secret_manager_secret.origin_shared_secret.secret_id
              version = "latest"
            }
          }
        }
      }
    }
  }

  lifecycle {
    ignore_changes = [
      # O startup probe usa run.googleapis.com/startupProbeType=Custom, campo que o
      # provider ainda não expõe — não deixar isso gerar diff eterno.
      template[0].labels,
      # Ignora a imagem para que o Terraform não desfaça o `just deploy-backend`
      template[0].containers[0].image,
      # Carimbos que o `gcloud run deploy --source` grava a cada deploy (quem deployou,
      # o build de origem, o scaling de serviço zerado). Nenhum é config deste
      # arquivo; sem isto, todo plan depois de um deploy pede para apagá-los.
      client,
      client_version,
      build_config,
      scaling,
    ]
  }

  # ORIGIN_SHARED_SECRET aponta para a versão "latest", e a referência acima é só ao
  # container do secret. Sem isto, a apply que liga a Cloudflare pode subir a revisão
  # antes de existir versão alguma, e a revisão não sobe.
  depends_on = [google_secret_manager_secret_version.origin_shared_secret]
}

# `--allow-unauthenticated` no deploy original virou este binding: qualquer um pode
# invocar. É intencional (API pública do app) — mudar isso é decisão com ADR, não
# ajuste de passagem (AGENTS.md, regra 9).
resource "google_cloud_run_v2_service_iam_member" "public_invoker" {
  name     = google_cloud_run_v2_service.logn.name
  location = google_cloud_run_v2_service.logn.location
  role     = "roles/run.invoker"
  member   = "allUsers"
}
