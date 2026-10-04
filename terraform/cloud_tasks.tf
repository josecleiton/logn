# Caixa de saída de e-mail (ADR 0026). O backend grava o e-mail em `email_outbox` e cria
# uma tarefa nesta fila; a fila chama POST /api/v1/internal/email/send na URL .run.app,
# com token OIDC da conta logn-tasks (admin.tf). O Scheduler varre de cinco em cinco
# minutos o que ficou sem tarefa (scheduler.tf).

resource "google_project_service" "cloud_tasks" {
  service            = "cloudtasks.googleapis.com"
  disable_on_destroy = false
}

resource "google_cloud_tasks_queue" "email" {
  name     = "email"
  location = var.region

  # O SMTP do Resend aceita dois envios por segundo no plano free. A fila segura acima
  # disso, em vez de o SMTP recusar e a tarefa gastar tentativa.
  rate_limits {
    max_dispatches_per_second = 2
    max_concurrent_dispatches = 2
  }

  # max_attempts tem de bater com OutboxMaxAttempts (backend, email_outbox.go): na
  # última, a rota desiste da linha em vez de pedir outra tentativa que não vem. As
  # esperas entre as cinco são 30 s, 1, 2 e 4 min: 7,5 min ao todo, dentro dos quinze do
  # código, e uma queda do SMTP de alguns minutos não esgota as tentativas.
  retry_config {
    max_attempts  = 5
    min_backoff   = "30s"
    max_backoff   = "300s"
    max_doublings = 5
  }

  depends_on = [google_project_service.cloud_tasks]
}

# O Cloud Run cria as tarefas. Só nesta fila, não no projeto.
resource "google_cloud_tasks_queue_iam_member" "runtime_enqueuer" {
  name     = google_cloud_tasks_queue.email.name
  location = google_cloud_tasks_queue.email.location
  role     = "roles/cloudtasks.enqueuer"
  member   = "serviceAccount:${var.service_account_email}"
}

# Tarefa com token OIDC de uma conta exige que quem a cria possa agir como ela. Só sobre
# logn-tasks: o Cloud Run não age como nenhuma outra conta.
resource "google_service_account_iam_member" "runtime_acts_as_tasks" {
  service_account_id = google_service_account.tasks.name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${var.service_account_email}"
}
