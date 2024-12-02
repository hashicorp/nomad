# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

terraform {
  required_providers {
    enos = {
      source  = "registry.terraform.io/hashicorp-forge/enos"
      version = ">= 0.2.3"
    }
  }
}

data "enos_artifactory_item" "nomad" {
  username = var.artifactory_username
  token    = var.artifactory_token
  host     = var.artifactory_host
  repo     = var.artifactory_repo
  path     = var.edition == "ce" ? "nomad/*" : "nomad-enterprise/*"
  name     = local.artifact_name
  properties = tomap({
    "commit"          = var.revision
    "product-name"    = var.edition == "ce" ? "nomad" : "nomad-enterprise"
//    "product-version" = local.artifact_version
  })
}

resource "enos_local_exec" "install_binary" {
    environment = {
        URL        = each.value.stdout
    }

    scripts = [abspath("${path.module}/scripts/install.sh")]
}