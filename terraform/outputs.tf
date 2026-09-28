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

output "cloudflare_nameservers" {
  description = "Nameservers que a Cloudflare atribuiu à zona — aponte o registrador do domínio pra eles."
  value       = var.enable_cloudflare ? cloudflare_zone.logn[0].name_servers : null
}

# O Terraform não cobre tudo sozinho: a URL .run.app crua continua respondendo depois
# do apply, porque o provider do Cloud Run (nesta versão) não expõe um jeito
# declarativo de desativá-la — é `gcloud run services update --no-default-url`, feito
# à mão, e só depois que o domain mapping estiver validado (senão você perde acesso ao
# serviço antes da Cloudflare estar servindo).
output "post_cloudflare_manual_steps" {
  description = "Passos que ficam de fora do Terraform depois do apply com enable_cloudflare=true."
  value = var.enable_cloudflare ? join("\n", [
    "1. Aponte o registrador de ${var.domain_name} para os nameservers do output cloudflare_nameservers.",
    "2. Espere o domain mapping ficar pronto: gcloud run domain-mappings describe --domain=${var.api_subdomain}.${var.domain_name} --region=${var.region}",
    "3. Confirme que https://${var.api_subdomain}.${var.domain_name}/health responde 200 pelo domínio novo.",
    "4. Só então desative a URL crua: gcloud run services update logn --region=${var.region} --no-default-url",
    "5. Aponte o app iOS para https://${var.api_subdomain}.${var.domain_name} em vez da URL .run.app.",
  ]) : null
}
