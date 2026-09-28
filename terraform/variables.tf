# Nada aqui tem default de produção. Todo valor identificável (projeto, contas de
# serviço, domínio, e-mail, URL do serviço) vem de terraform.tfvars, que é local e
# nunca entra no git — mesma regra do .env. O terraform.tfvars.example mostra a forma,
# com placeholder, e esse sim é publicável.

variable "project_id" {
  description = "ID do projeto no Google Cloud."
  type        = string
}

variable "region" {
  description = "Região onde o Cloud Run e o Cloud Scheduler rodam."
  type        = string
  default     = "us-east1"
}

variable "service_account_email" {
  description = "Service account que o Cloud Run e o Cloud Scheduler usam para se identificar."
  type        = string
}

variable "service_audience_url" {
  description = <<-EOT
    URL do serviço no Cloud Run, usada como audiência do OIDC (rota interna de purga) e
    como alvo do Cloud Scheduler. É a URL "críptica" default (https://logn-xxxxx.a.run.app),
    não o domínio custom — troca só se o serviço for recriado do zero.
  EOT
  type        = string
}

variable "smtp_user" {
  description = "Usuário SMTP (conta de e-mail que envia OTP e notificação)."
  type        = string
}

variable "smtp_from" {
  description = "Remetente exibido nos e-mails enviados pelo backend."
  type        = string
}

variable "smtp_host" {
  description = "Host do servidor SMTP."
  type        = string
  default     = "smtp.gmail.com"
}

variable "smtp_port" {
  description = "Porta do servidor SMTP."
  type        = string
  default     = "587"
}

variable "container_image" {
  description = <<-EOT
    Imagem já publicada no Artifact Registry que o Cloud Run deve rodar (digest ou
    tag). Terraform não builda nem sobe imagem — isso continua com
    `gcloud builds submit` (ou `just deploy-backend`), que deixa o resultado no
    Artifact Registry antes do `terraform apply`.
  EOT
  type        = string
}

# --- Verificação de origem (proxy confiável na frente do serviço) ---
# Ficam com default vazio de propósito: o backend (origin_verification.go) falha
# fechado em produção se ORIGIN_TRUSTED_CIDRS/ORIGIN_SHARED_SECRET não estiverem
# setados. Só preenche depois que o domínio custom + Cloudflare estiverem prontos;
# até lá, essas variáveis ficam de fora do var-file e o Cloud Run continua exatamente
# como está hoje.

variable "origin_trusted_cidrs" {
  description = <<-EOT
    CIDRs do proxy confiável, separados por vírgula — só usado quando
    enable_cloudflare=false (proxy que não seja a Cloudflare, preenchido à mão). Com
    Cloudflare ligada, os CIDRs vêm sozinhos do data source cloudflare_ip_ranges, e
    esta variável é ignorada.
  EOT
  type        = string
  default     = ""
}

variable "enable_origin_verification" {
  description = "Liga a checagem de origem no Cloud Run (env ORIGIN_TRUSTED_CIDRS e o secret ORIGIN_SHARED_SECRET)."
  type        = bool
  default     = false
}

# --- Cloudflare (proxy na frente do Cloud Run) ---
# Tudo aqui com default neutro: enable_cloudflare=false não cria zona, não toca DNS, não
# gera segredo. Liga junto com enable_origin_verification quando o domínio estiver
# registrado e adicionado à conta Cloudflare.

variable "enable_cloudflare" {
  description = "Liga os recursos da Cloudflare (DNS, domain mapping, Transform Rule do segredo, Bot Fight Mode) na zona já existente."
  type        = bool
  default     = false
}

variable "cloudflare_api_token" {
  description = <<-EOT
    API Token da Cloudflare (não o Global API Key) com permissão de Zone/DNS/Ruleset
    edit para o domínio. Nunca versionado. O default é um placeholder sem validade —
    passa a checagem de formato do provider (só letras/números/hífen/underscore), mas
    não autentica; só importa de verdade quando enable_cloudflare=true.
  EOT
  type        = string
  default     = "unset0placeholder0not0a0real0token000000"
  sensitive   = true
}

variable "domain_name" {
  description = "Domínio raiz registrado (ex.: logn.sh)."
  type        = string
  default     = ""
}

variable "api_subdomain" {
  description = "Subdomínio que aponta para o backend no Cloud Run — a raiz fica livre para uma futura landing page."
  type        = string
  default     = "api"
}
