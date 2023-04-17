# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

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
