# As contas que assinam as rotas internas (ADR 0021). Nenhuma tem papel no projeto nem
# chave: existem só para assinar o token OIDC de uma rota, e o backend aceita, em cada
# rota, só o token cujo e-mail é o da conta dela (cloud_run.tf).

# A purga. Era a conta com que o Cloud Run roda, e aí o próprio servidor, ou quem o
# fizesse buscar uma URL, emitia token de purga pelo servidor de metadados.
resource "google_service_account" "scheduler" {
  account_id   = "logn-scheduler"
  display_name = "Cloud Scheduler: purga de contas (rota interna)"
}

# A revogação manual de licença e a contestação. Quem está em admin_members emite o
# token em nome dela pelo gcloud (`just revoke`), com a própria conta Google.
resource "google_service_account" "admin" {
  account_id   = "logn-admin"
  display_name = "Administração das licenças (rotas internas)"
}

# Só o token de identidade, e só sobre esta conta, nunca no projeto. O TokenCreator
# daria também token de acesso e assinatura de qualquer coisa em nome dela. A API de IAM
# Credentials já está ligada pelo deploy (github_deploy.tf).
resource "google_service_account_iam_member" "admin_token_creators" {
  for_each           = toset(var.admin_members)
  service_account_id = google_service_account.admin.name
  role               = "roles/iam.serviceAccountOpenIdTokenCreator"
  member             = each.value
}
