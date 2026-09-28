# Deploy do backend pelo GitHub Actions (ADR 0015).
#
# O workflow roda no repositório de conteúdo, que é privado: é ele que junta o código
# do app com as migrações de conteúdo, que não entram no repositório público. O GitHub
# prova quem é por OIDC (Workload Identity Federation) — não existe chave JSON em
# segredo nenhum. Só passa o token do repositório de id `github_deploy_repository_id`
# (o id numérico não se recria apagando e refazendo o nome), da branch
# `github_deploy_ref`, do arquivo de workflow de deploy e do environment `production`.
#
# Duas contas, cada uma com o mínimo:
#   - `logn-deployer`, que o GitHub assume: sobe o código e cria a revisão
#     (roles/run.sourceDeveloper, que não lê o Secret Manager direto) e age como a
#     conta de build e como a conta com que o serviço roda. Agir como a de runtime é
#     poder rodar código com as permissões dela: o deployer vale o que ela vale.
#   - `logn-builder`, com que o Cloud Build roda: escreve a imagem, lê o código enviado
#     e grava log. O build rodava com a conta padrão do Compute, que tem roles/editor
#     no projeto; agir como ela daria ao CI o projeto inteiro.

data "google_project" "current" {}

# A troca do token do GitHub pelo do Google passa pelo STS, e a impersonação do
# deployer pelo IAM Credentials. O STS não vinha ligado no projeto.
resource "google_project_service" "github_deploy" {
  for_each = toset(["sts.googleapis.com", "iamcredentials.googleapis.com"])

  service            = each.key
  disable_on_destroy = false
}

resource "google_iam_workload_identity_pool" "github" {
  workload_identity_pool_id = "github-actions"
  display_name              = "GitHub Actions"
  description               = "Deploy do backend a partir do repositório de conteúdo (ADR 0015)."

  depends_on = [google_project_service.github_deploy]
}

resource "google_iam_workload_identity_pool_provider" "github" {
  workload_identity_pool_id          = google_iam_workload_identity_pool.github.workload_identity_pool_id
  workload_identity_pool_provider_id = "github"
  display_name                       = "GitHub OIDC"

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }

  attribute_mapping = {
    "google.subject"          = "assertion.sub"
    "attribute.repository_id" = "assertion.repository_id"
    "attribute.ref"           = "assertion.ref"
    "attribute.workflow"      = "assertion.job_workflow_ref"
    "attribute.environment"   = "assertion.environment"
  }

  # Sem condição, qualquer repositório do GitHub trocaria token neste pool. Com ela, só
  # o job de deploy da main, no environment production, de um repositório só.
  attribute_condition = join(" && ", [
    "assertion.repository_id == \"${var.github_deploy_repository_id}\"",
    "assertion.ref == \"${var.github_deploy_ref}\"",
    "assertion.job_workflow_ref == \"${var.github_deploy_repository}/.github/workflows/deploy-backend.yml@${var.github_deploy_ref}\"",
    "assertion.environment == \"production\"",
  ])
}

resource "google_service_account" "deployer" {
  account_id   = "logn-deployer"
  display_name = "Deploy do backend pelo GitHub Actions"
}

resource "google_service_account" "builder" {
  account_id   = "logn-builder"
  display_name = "Build do backend no Cloud Build"
}

# O GitHub, e só pelo repositório e branch da condição acima, assume o deployer.
resource "google_service_account_iam_member" "github_assumes_deployer" {
  service_account_id = google_service_account.deployer.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github.name}/attribute.repository_id/${var.github_deploy_repository_id}"
}

resource "google_project_iam_member" "deployer_source_deploy" {
  project = var.project_id
  role    = "roles/run.sourceDeveloper"
  member  = "serviceAccount:${google_service_account.deployer.email}"
}

# A revisão nova roda com a conta de sempre, e o build com a conta própria.
resource "google_service_account_iam_member" "deployer_acts_as_runtime" {
  service_account_id = "projects/${var.project_id}/serviceAccounts/${var.service_account_email}"
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.deployer.email}"
}

resource "google_service_account_iam_member" "deployer_acts_as_builder" {
  service_account_id = google_service_account.builder.name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.deployer.email}"
}

# O repositório de imagem e o bucket de código que o `gcloud run deploy --source` usa.
# Os nomes são a convenção do gcloud; os dois já existem dos deploys feitos à mão.
resource "google_artifact_registry_repository_iam_member" "builder_pushes_image" {
  location   = var.region
  repository = "cloud-run-source-deploy"
  role       = "roles/artifactregistry.writer"
  member     = "serviceAccount:${google_service_account.builder.email}"
}

resource "google_storage_bucket_iam_member" "builder_reads_source" {
  bucket = "run-sources-${var.project_id}-${var.region}"
  role   = "roles/storage.objectViewer"
  member = "serviceAccount:${google_service_account.builder.email}"
}

resource "google_project_iam_member" "builder_writes_logs" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.builder.email}"
}
