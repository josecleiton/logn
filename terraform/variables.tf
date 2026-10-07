# Nada aqui tem default de produção. Todo valor identificável (projeto, contas de
# serviço, domínio, e-mail, URL do serviço) vem de terraform.tfvars, que nunca entra no
# git — mesma regra do .env. A cópia de verdade fica no bucket do state (`just
# tfvars-pull` / `just tfvars-push`). O terraform.tfvars.example mostra a forma, com
# placeholder, e esse sim é publicável.

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
  description = "Service account com que o Cloud Run roda. O Cloud Scheduler tem a sua (admin.tf)."
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

variable "admin_members" {
  description = <<-EOT
    Quem pode emitir token em nome da conta logn-admin e chamar as rotas internas de
    licença (ADR 0021), no formato do IAM ("user:voce@example.com"). Vazio, ninguém chama.
  EOT
  type        = list(string)
  default     = []
}

variable "play_publishers" {
  description = <<-EOT
    Quem pode emitir token em nome da conta logn-play-publisher e subir build ao teste
    interno do Play (`just android/play-internal`), no formato do IAM
    ("user:voce@example.com"). Vazio, ninguém sobe.
  EOT
  type        = list(string)
  default     = []
}

variable "github_deploy_repository" {
  description = "Repositório do GitHub (dono/nome) cujo workflow pode fazer deploy — o de conteúdo, privado (ADR 0015)."
  type        = string
}

variable "github_deploy_repository_id" {
  description = "Id numérico desse repositório (gh api repos/<dono>/<nome> --jq .id). Não muda com renomeação nem volta se o nome for recriado."
  type        = string
}

variable "github_deploy_ref" {
  description = "Única branch desse repositório que pode fazer deploy."
  type        = string
  default     = "refs/heads/main"
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

# 465 é TLS implícito. Em qualquer outra porta a cifra vem do STARTTLS, que em produção
# o backend exige (ADR 0027); 465 dispensa depender dessa oferta.
variable "smtp_port" {
  description = "Porta do servidor SMTP."
  type        = string
  default     = "465"
}

variable "github_client_id" {
  description = <<-EOT
    Client ID do OAuth App do GitHub (ADR 0019). Não é segredo; o secret dele vai em
    logn-server-keys. Os dois valem juntos: vazio aqui com o secret lá, ou o contrário,
    o servidor não sobe. Vazio nos dois desliga o login com GitHub.
  EOT
  type        = string
  default     = ""
}

variable "google_ios_client_id" {
  description = <<-EOT
    Client ID do app iOS no Google Auth Platform (termina em .apps.googleusercontent.com).
    É a audiência que o backend exige no ID token do login com Google (ADR 0016). Não é
    segredo, mas identifica o projeto e fica fora do git como o resto. Vazio desliga o
    login com Google no servidor, que sobe do mesmo jeito.
  EOT
  type        = string
  default     = ""
}

# --- Android e Google Play (ADR 0022) ---

variable "google_web_client_id" {
  description = <<-EOT
    Client ID do tipo "Aplicativo da Web" no Google Auth Platform. No Android, o
    Credential Manager pede o ID token em nome dele (serverClientId), e ele é o `aud` do
    token. Vem junto com google_android_client_ids, ou o backend não sobe.
  EOT
  type        = string
  default     = ""
}

variable "google_android_client_ids" {
  description = <<-EOT
    Clients ID do tipo "Android" (pacote + SHA-1 da chave que assina), um por chave: a
    de debug, a de upload e a do Play App Signing, que o Play Console mostra depois do
    primeiro upload. Cada um é um `azp` aceito no ID token do Android, e é o que separa
    o nosso app de qualquer outro client do projeto.
  EOT
  type        = list(string)
  default     = []

  # Um sem o outro o Terraform deixaria de mandar os dois, e o login do Android sumiria
  # calado; o backend recusa subir com meio par, e aqui também.
  validation {
    condition     = (length(var.google_android_client_ids) == 0) == (var.google_web_client_id == "")
    error_message = "google_web_client_id e google_android_client_ids vêm juntos."
  }
}

variable "play_package_name" {
  description = <<-EOT
    Nome do pacote do app no Google Play. Liga a compra pelo Play e o job diário das
    compras anuladas. A conta de serviço do Cloud Run precisa estar convidada no Play
    Console (Usuários e permissões) com "Ver dados financeiros" e "Gerenciar pedidos";
    isso não é Terraform. Vazio desliga os dois.
  EOT
  type        = string
  default     = ""
}

# --- Sign in with Apple (ADR 0017) ---
# Tudo desligado até existir a chave, que só a conta paga do Apple Developer gera
# (Certificates, Identifiers & Profiles > Keys, com "Sign in with Apple").

variable "enable_apple_signin" {
  description = "Liga o Sign in with Apple no Cloud Run. Só depois de subir a .p8 para o secret logn-apple-signin-key."
  type        = bool
  default     = false
}

variable "apple_signin_team_id" {
  description = "Team ID do Apple Developer (10 caracteres), emissor do client_secret da revogação."
  type        = string
  default     = ""
}

variable "apple_signin_key_id" {
  description = "Key ID da chave do Sign in with Apple (10 caracteres), no cabeçalho do client_secret."
  type        = string
  default     = ""
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
  description = "Liga os recursos da Cloudflare (DNS, domain mapping, Transform Rule do segredo, rate limit de auth, Bot Fight Mode) na zona já existente."
  type        = bool
  default     = false
}

variable "cloudflare_api_token" {
  description = <<-EOT
    API Token da Cloudflare (não o Global API Key), restrito à zona do domínio, com:
    Zone Read, DNS Edit, Transform Rules Edit, WAF Edit (rate limit), Bot
    Management Edit e Zone Settings Edit (Email Obfuscation desligado). Nunca versionado. O default é um placeholder sem validade —
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

variable "enable_waitlist" {
  description = "Liga a lista de espera do iPhone (ADR 0022): as rotas /api/v1/waitlist respondem com 303 para a landing em domain_name. Só depois de a política de privacidade cobrir a lista."
  type        = bool
  default     = false
}
