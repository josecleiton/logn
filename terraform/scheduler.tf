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

    oidc_token {
      service_account_email = var.service_account_email
      audience              = local.scheduler_audience
    }
  }
}
