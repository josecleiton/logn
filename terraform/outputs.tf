output "cloud_run_url" {
  description = "URL default do serviço no Cloud Run."
  value       = google_cloud_run_v2_service.logn.uri
}

output "admin_service_account" {
  description = "Conta em nome da qual o `just revoke` emite o token das rotas internas de licença (ADR 0021)."
  value       = google_service_account.admin.email
}

output "secret_ids" {
  description = "Nomes dos secrets que o Terraform criou — o valor sobe fora daqui, com `gcloud secrets versions add`."
  value = [
    google_secret_manager_secret.db_url.secret_id,
    google_secret_manager_secret.server_keys.secret_id,
    google_secret_manager_secret.smtp_pass.secret_id,
    google_secret_manager_secret.origin_shared_secret.secret_id,
  ]
}

# Os três valores que o workflow de deploy lê como variáveis do repositório de conteúdo
# (Settings › Variables). Nenhum é segredo: sem passar na condição do provider, não
# servem para nada.
output "github_deploy_variables" {
  description = "Variáveis do repositório de conteúdo para o workflow de deploy (ADR 0015)."
  value = {
    GCP_WORKLOAD_IDENTITY_PROVIDER = google_iam_workload_identity_pool_provider.github.name
    GCP_DEPLOYER_SERVICE_ACCOUNT   = google_service_account.deployer.email
    GCP_BUILDER_SERVICE_ACCOUNT    = google_service_account.builder.id
  }
}

output "cloudflare_nameservers" {
  description = "Nameservers que a Cloudflare atribuiu à zona — aponte o registrador do domínio pra eles."
  value       = var.enable_cloudflare ? data.cloudflare_zone.logn[0].name_servers : null
}

# O Terraform não cobre tudo sozinho. A URL .run.app crua fica ligada de propósito: é
# por ela que o Cloud Scheduler chama a purga, fora do Bot Fight Mode (scheduler.tf).
# Com a verificação de origem ligada, ela só responde às rotas de originExempt
# (main.go) — health e a rota interna, que exige OIDC.
output "post_cloudflare_manual_steps" {
  description = "Passos que ficam de fora do Terraform depois do apply com enable_cloudflare=true."
  value = var.enable_cloudflare ? join("\n", [
    "1. Confira que o registrador de ${var.domain_name} aponta para os nameservers do output cloudflare_nameservers.",
    "2. Espere o domain mapping ficar pronto: gcloud beta run domain-mappings describe --domain=${var.api_subdomain}.${var.domain_name} --region=${var.region}",
    "3. Confirme que https://${var.api_subdomain}.${var.domain_name}/health responde 200 pelo domínio novo.",
    "4. Aponte o app iOS para https://${var.api_subdomain}.${var.domain_name} em vez da URL .run.app.",
    "5. Não rode --no-default-url: o Cloud Scheduler depende da URL .run.app.",
  ]) : null
}
