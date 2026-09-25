# Tudo aqui é condicional a enable_cloudflare (default false) — com a flag desligada,
# nenhum recurso deste arquivo existe, e terraform apply nem toca na conta Cloudflare.
#
# Pré-requisito manual que o Terraform não automatiza: antes da primeira apply com
# enable_cloudflare=true, o domínio precisa estar verificado no Google Search Console
# (https://search.google.com/search-console) com a mesma conta usada no `gcloud auth
# application-default login` — sem isso, google_cloud_run_domain_mapping falha com
# "domain not verified". É verificação de posse feita uma vez, não repete a cada apply.

resource "cloudflare_zone" "logn" {
  count = var.enable_cloudflare ? 1 : 0

  account = { id = var.cloudflare_account_id }
  name    = var.domain_name
}

# Liga o subdomínio (api.<domínio>) ao serviço já existente no Cloud Run. A raiz do
# domínio fica de fora de propósito — sobra livre para uma landing page futura, sem
# competir por rota com a API.
resource "google_cloud_run_domain_mapping" "api" {
  count = var.enable_cloudflare ? 1 : 0

  location = var.region
  name     = "${var.api_subdomain}.${var.domain_name}"

  metadata {
    namespace = var.project_id
  }

  spec {
    route_name       = google_cloud_run_v2_service.logn.name
    certificate_mode = "AUTOMATIC"
  }
}

# O Google devolve o(s) registro(s) DNS que o mapeamento espera (normalmente um CNAME
# para ghs.googlehosted.com) — replica cada um na Cloudflare, com proxy ligado
# (proxied=true: é isso que coloca a Cloudflare no meio do caminho).
resource "cloudflare_dns_record" "api" {
  for_each = var.enable_cloudflare ? {
    for r in google_cloud_run_domain_mapping.api[0].status[0].resource_records :
    r.name => r
  } : {}

  zone_id = cloudflare_zone.logn[0].id
  name    = var.api_subdomain
  type    = each.value.type
  content = each.value.rrdata
  ttl     = 1 # "automático" na Cloudflare
  proxied = true
}

# Ranges de IP públicos da Cloudflare, direto da fonte — nunca hardcoded, nunca
# desatualizado. É o que origin_verification.go compara contra o X-Forwarded-For.
#
# Com count: sem isso, o data source roda em todo `terraform plan`, mesmo com
# enable_cloudflare=false — quebraria qualquer apply da parte GCP sem token da
# Cloudflare configurado.
data "cloudflare_ip_ranges" "this" {
  count = var.enable_cloudflare ? 1 : 0
}

locals {
  cloudflare_cidrs = var.enable_cloudflare ? join(",", concat(
    data.cloudflare_ip_ranges.this[0].ipv4_cidrs,
    data.cloudflare_ip_ranges.this[0].ipv6_cidrs,
  )) : ""
}

# Segredo que a Cloudflare injeta em todo request e o backend confere — gerado aqui,
# não à mão: assim os dois lados (Transform Rule e Secret Manager) nunca divergem. Vive
# no state (local, fora do git, mesma proteção do resto).
resource "random_password" "origin_shared_secret" {
  count   = var.enable_cloudflare ? 1 : 0
  length  = 40
  special = false
}

resource "google_secret_manager_secret_version" "origin_shared_secret" {
  count       = var.enable_cloudflare ? 1 : 0
  secret      = google_secret_manager_secret.origin_shared_secret.id
  secret_data = random_password.origin_shared_secret[0].result
}

# Transform Rule: injeta o header antes do request chegar no Cloud Run. Fase
# http_request_late_transform roda depois de toda regra de cache/redirect, logo antes
# de sair da borda — é o mais perto possível do request real que o origin vê.
resource "cloudflare_ruleset" "origin_header" {
  count = var.enable_cloudflare ? 1 : 0

  zone_id = cloudflare_zone.logn[0].id
  name    = "logn-origin-verify-header"
  kind    = "zone"
  phase   = "http_request_late_transform"

  rules = [
    {
      action      = "rewrite"
      description = "Injeta o segredo que o backend confere em origin_verification.go"
      enabled     = true
      expression  = "true"
      action_parameters = {
        headers = {
          "X-Origin-Verify" = {
            operation = "set"
            value     = random_password.origin_shared_secret[0].result
          }
        }
      }
    }
  ]
}

# Bot Fight Mode — grátis no plano free, mitigação básica de bot antes de chegar na
# aplicação.
resource "cloudflare_bot_management" "logn" {
  count      = var.enable_cloudflare ? 1 : 0
  zone_id    = cloudflare_zone.logn[0].id
  fight_mode = true
}
