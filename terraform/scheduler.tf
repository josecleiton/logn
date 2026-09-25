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

  http_target {
    http_method = "POST"
    uri         = "${var.service_audience_url}/api/v1/internal/purge"

    oidc_token {
      service_account_email = var.service_account_email
      audience              = var.service_audience_url
    }
  }
}
