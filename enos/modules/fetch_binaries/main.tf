# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

terraform {
  required_providers {
    enos = {
      source = "registry.terraform.io/hashicorp-forge/enos"
    }
  }
}

module fetch_artifact {
  for_each = toset(distinct(var.oss))
  source   = "../fetch_artifactory"

  artifactory_credentials = {
    username = var.artifactory_username
    token    = var.artifactory_token
  }

  artifactory_host = var.artifactory_host
  artifactory_repo = var.artifactory_repo

  binary_config = {
    edition         = var.edition
    os              = each.value
    product_version = var.product_version
    arch            = var.arch
  }

  download_binary      = var.download_binaries
  download_binary_path = "${var.download_binaries_path}/${each.value}"
}
