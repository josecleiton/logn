terraform {
  required_version = ">= 1.5.0"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 5.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }

  # Estado num bucket do GCS, versionado e privado, em us-east1: dentro do Always Free.
  # Local, ele vivia só numa máquina, e perder o arquivo era o Terraform achar que nada
  # existe. O bucket (`<projeto>-tfstate`) não é gerenciado aqui, porque guarda o
  # próprio estado; foi criado à mão, e o nome entra no init: `just tf-init`. O
  # terraform.tfvars mora no mesmo bucket (`just tfvars-pull` / `tfvars-push`).
  backend "gcs" {
    prefix = "terraform/state"
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

provider "cloudflare" {
  api_token = var.cloudflare_api_token
}
