output "cloud_run_url" {
  description = "URL default do serviço no Cloud Run."
  value       = google_cloud_run_v2_service.logn.uri
}

output "secret_ids" {
  description = "Nomes dos secrets que o Terraform criou — o valor sobe fora daqui, com `gcloud secrets versions add`."
  value = [
    google_secret_manager_secret.db_url.secret_id,
    google_secret_manager_secret.jwt_secret.secret_id,
    google_secret_manager_secret.smtp_pass.secret_id,
    google_secret_manager_secret.origin_shared_secret.secret_id,
  ]
}
