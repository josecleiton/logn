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
  description = "CIDRs do proxy confiável (ex.: ranges publicados da Cloudflare), separados por vírgula. Vazio = verificação de origem desligada."
  type        = string
  default     = ""
}

variable "enable_origin_verification" {
  description = <<-EOT
    Liga a checagem de origem no Cloud Run (env ORIGIN_TRUSTED_CIDRS e a referência ao
    secret ORIGIN_SHARED_SECRET). O valor do segredo nunca passa por variável do
    Terraform — sobe direto no Secret Manager por `gcloud secrets versions add`, e o
    container de secret que este módulo cria (google_secret_manager_secret.origin_shared_secret)
    fica vazio até isso acontecer.
  EOT
  type        = bool
  default     = false
}
