terraform {
  required_providers {
    digitalocean = {
      source = "digitalocean/digitalocean"
    }
    nomad = {
      source = "hashicorp/nomad"
    }
  }
  required_version = ">= 0.13"
}
