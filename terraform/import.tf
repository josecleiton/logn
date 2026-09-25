# Traz os recursos que já existem (criados na mão) para debaixo do Terraform, sem
# recriar nada. `terraform plan` com isso tem que dar zero para adicionar e zero para
# destruir — só "importar". Depois que `terraform apply` rodar uma vez com sucesso,
# este arquivo não serve mais para nada (o estado já sabe da associação) e pode sair.

import {
  to = google_cloud_run_v2_service.logn
  id = "projects/${var.project_id}/locations/${var.region}/services/logn"
}

import {
  to = google_cloud_run_v2_service_iam_member.public_invoker
  id = "projects/${var.project_id}/locations/${var.region}/services/logn roles/run.invoker allUsers"
}

import {
  to = google_cloud_scheduler_job.purge_deleted_accounts
  id = "projects/${var.project_id}/locations/${var.region}/jobs/purge-deleted-accounts"
}

import {
  to = google_secret_manager_secret.db_url
  id = "projects/${var.project_id}/secrets/logn-db-url"
}

import {
  to = google_secret_manager_secret.jwt_secret
  id = "projects/${var.project_id}/secrets/logn-jwt-secret"
}

import {
  to = google_secret_manager_secret.smtp_pass
  id = "projects/${var.project_id}/secrets/logn-smtp-pass"
}

# Este não existe ainda no Secret Manager — cria na primeira apply, não importa.

import {
  to = google_secret_manager_secret.db_migrator_url
  id = "projects/${var.project_id}/secrets/logn-db-migrator-url"
}
