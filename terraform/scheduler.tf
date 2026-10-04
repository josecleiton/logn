resource "google_cloud_scheduler_job" "purge_deleted_accounts" {
  name             = "purge-deleted-accounts"
  region           = var.region
  schedule         = "0 3 * * *"
  time_zone        = "Etc/UTC"
  attempt_deadline = "180s"

  retry_config {
    min_backoff_duration = "5s"
    max_backoff_duration = "3600s"
    max_doublings        = 5
  }

  # Mira a URL .run.app, não o domínio custom, também com Cloudflare ligada: o Bot
  # Fight Mode da zona desafiaria o Scheduler e não aceita exceção no plano free. O
  # backend isenta /api/v1/internal/* da verificação de origem (main.go), e a rota
  # segue protegida pelo OIDC. Por isso a URL crua não pode ser desligada
  # (--no-default-url). local.scheduler_audience está em cloud_run.tf, que usa o mesmo
  # valor no CLOUD_SCHEDULER_AUDIENCE do backend; os dois têm que bater ou o OIDC falha.
  http_target {
    http_method = "POST"
    uri         = "${local.scheduler_audience}/api/v1/internal/purge"

    # Conta própria, não a do Cloud Run (admin.tf): o backend só aceita a purga assinada
    # por ela (CLOUD_SCHEDULER_SERVICE_ACCOUNT, ADR 0021).
    oidc_token {
      service_account_email = google_service_account.scheduler.email
      audience              = local.scheduler_audience
    }
  }
}

# Varredura da caixa de saída de e-mail (ADR 0026): põe na fila o que o pedido não
# conseguiu pôr. Mesma URL .run.app e mesma conta da purga. Sem nova tentativa: a
# próxima varredura roda em cinco minutos. É o terceiro job, o último do free tier.
resource "google_cloud_scheduler_job" "email_outbox_sweep" {
  name             = "email-outbox-sweep"
  region           = var.region
  schedule         = "*/5 * * * *"
  time_zone        = "Etc/UTC"
  attempt_deadline = "60s"

  retry_config {
    retry_count = 0
  }

  http_target {
    http_method = "POST"
    uri         = "${local.scheduler_audience}/api/v1/internal/email/sweep"

    oidc_token {
      service_account_email = google_service_account.scheduler.email
      audience              = local.scheduler_audience
    }
  }
}

# A Google Play Developer API não vem ligada no projeto; sem ela, toda consulta de
# compra responde 403, e o backend responde 502 à compra (ADR 0022).
resource "google_project_service" "android_publisher" {
  count              = var.play_package_name != "" ? 1 : 0
  service            = "androidpublisher.googleapis.com"
  disable_on_destroy = false
}

# Reembolso e estorno do Google Play (ADR 0022): uma vez por dia, a rota interna lê a
# Voided Purchases API e revoga. Mesma URL .run.app e mesma conta da purga; a janela da
# consulta é de 29 dias, então um dia sem rodar não perde nada.
resource "google_cloud_scheduler_job" "play_voided_purchases" {
  count            = var.play_package_name != "" ? 1 : 0
  name             = "play-voided-purchases"
  region           = var.region
  schedule         = "30 3 * * *"
  time_zone        = "Etc/UTC"
  attempt_deadline = "180s"

  retry_config {
    min_backoff_duration = "5s"
    max_backoff_duration = "3600s"
    max_doublings        = 5
  }

  http_target {
    http_method = "POST"
    uri         = "${local.scheduler_audience}/api/v1/internal/play/voided"

    oidc_token {
      service_account_email = google_service_account.scheduler.email
      audience              = local.scheduler_audience
    }
  }
}
