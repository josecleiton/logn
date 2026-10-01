# A conta que sobe o .aab para o teste interno do Play (`just android/play-internal`).
# Não tem papel no projeto nem chave: o que ela pode está no Play Console, onde é
# convidada à mão (Usuários e permissões › só este app › "Lançar em faixas de teste").
# Separada da conta do Cloud Run, que lê compras: o servidor não lança versão.

resource "google_service_account" "play_publisher" {
  account_id   = "logn-play-publisher"
  display_name = "Envio de builds ao teste interno do Google Play"
}

# O token de acesso, e só sobre esta conta, nunca no projeto. Quem está em
# play_publishers emite o token em nome dela pelo IAM Credentials, com a própria conta
# Google do gcloud; nenhuma chave sai do Google. A API de IAM Credentials já está ligada
# pelo deploy (github_deploy.tf).
resource "google_service_account_iam_member" "play_publisher_token_creators" {
  for_each           = toset(var.play_publishers)
  service_account_id = google_service_account.play_publisher.name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = each.value
}
