terraform {
  required_providers {
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "1.26.0"
    }
    nomad = {
      source = "hashicorp/nomad"
    }
  }
}
