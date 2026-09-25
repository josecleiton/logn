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

  # Com Cloudflare ligada, mira o domínio custom em vez da URL .run.app — assim o
  # request passa pelo proxy (Scheduler -> Cloudflare -> Cloud Run) em vez de bater
  # direto no serviço. O backend continua isentando /api/v1/internal/* da verificação
  # de origem (main.go) — essa rota segue protegida só pelo OIDC, como hoje; passar
  # pelo proxy é sobre não depender mais da URL crua, não sobre reforço extra aqui.
  # Antes de ligar a Cloudflare, ou com outro proxy, mira a URL default mesmo
  # (local.scheduler_audience cobre os dois casos — ver cloud_run.tf, que usa o mesmo
  # valor no CLOUD_SCHEDULER_AUDIENCE do backend; os dois têm que bater ou o OIDC falha).
  http_target {
    http_method = "POST"
    uri         = "${local.scheduler_audience}/api/v1/internal/purge"

    oidc_token {
      service_account_email = var.service_account_email
      audience              = local.scheduler_audience
    }
  }
}
