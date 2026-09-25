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

  # Estado local, de propósito: MVP solo, sem time nem CI aplicando em paralelo. O
  # arquivo de estado (terraform.tfstate) não entra no git — reflete o projeto real,
  # e ninguém de fora do dono precisa dele. Se isso crescer (mais gente aplicando,
  # ou CI/CD), o primeiro passo é migrar pra um backend remoto (GCS bucket), não
  # antes.
}

provider "google" {
  project = var.project_id
  region  = var.region
}

provider "cloudflare" {
  api_token = var.cloudflare_api_token
}
