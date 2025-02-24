# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

terraform {
  required_providers {
    enos = {
      source = "registry.terraform.io/hashicorp-forge/enos"
    }
  }
}

locals {

  path = var.binary_config.edition == "ce" ? "nomad/*" : "nomad-enterprise/*"

  artifact_version = var.binary_config.edition == "ce" ? "${var.binary_config.product_version}" : "${var.binary_config.product_version}+ent"

  package_extensions = {
    amd64 = {
      linux   = "_linux_amd64.zip"
      windows = "_windows_amd64.zip"
    }

    arm64 = {
      linux = "_linux_arm64.zip"
    }
  }

  artifact_name = "nomad_${local.artifact_version}${local.package_extensions[var.binary_config.arch][var.binary_config.os]}"
  artifact_zip  = "${local.artifact_name}.zip"
  local_binary  = var.binary_config.os == "windows" ? "${var.download_binary_path}/nomad.exe" : "${var.download_binary_path}/nomad"
}

data "enos_artifactory_item" "nomad" {
  username = var.artifactory_credentials.username
  token    = var.artifactory_credentials.token
  host     = var.artifactory_host
  repo     = var.artifactory_repo
  path     = local.path
  name     = local.artifact_name

  properties = tomap({
    "product-name" = var.binary_config.edition == "ce" ? "nomad" : "nomad-enterprise"
  })
}

resource "enos_local_exec" "install_binary" {
  count = var.download_binary ? 1 : 0

  environment = {
    URL         = data.enos_artifactory_item.nomad.results[0].url
    BINARY_PATH = var.download_binary_path
    TOKEN       = var.artifactory_credentials.token
    LOCAL_ZIP   = local.artifact_zip
  }

  scripts = [abspath("${path.module}/scripts/install.sh")]
}
