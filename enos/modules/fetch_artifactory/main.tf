# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

terraform {
  required_providers {
    enos = {
      source = "registry.terraform.io/hashicorp-forge/enos"
    }
  }
}

data "enos_artifactory_item" "nomad" {
  username = var.artifactory_username
  token    = var.artifactory_token
  host     = var.artifactory_host
  repo     = var.artifactory_repo
  path     = local.path
  name     = local.artifact_name
  properties = tomap({
    "product-name" = var.edition == "ce" ? "nomad" : "nomad-enterprise"
  })
}

resource "enos_local_exec" "install_binary" {
  environment = {
    URL         = data.enos_artifactory_item.nomad.results[0].url
    BINARY_PATH = var.binary_path
    TOKEN       = var.artifactory_token
  }

  scripts = [abspath("${path.module}/scripts/install.sh")]
}  