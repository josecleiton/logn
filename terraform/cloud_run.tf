locals {
  # Alvo da rota interna de purga (Cloud Scheduler + a env que o backend valida). Com
  # Cloudflare ligada, vira o domínio custom — o request passa pelo proxy e a rota
  # interna deixa de precisar de exceção na verificação de origem. Scheduler.tf usa o
  # mesmo valor; os dois têm que bater ou o OIDC falha.
  scheduler_audience = var.enable_cloudflare ? "https://${var.api_subdomain}.${var.domain_name}" : var.service_audience_url

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
      env {
        name  = "RUN_MIGRATIONS"
        value = "true"
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
      env {
        name = "JWT_SECRET"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.jwt_secret.secret_id
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
    ]
  }
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
