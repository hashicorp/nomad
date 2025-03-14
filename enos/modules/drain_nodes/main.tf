# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

terraform {
  required_providers {
    enos = {
      source = "hashicorp-forge/enos"
    }
  }
}

locals {
  nomad_env = {
    NOMAD_ADDR        = var.nomad_addr
    NOMAD_CACERT      = var.ca_file
    NOMAD_CLIENT_CERT = var.cert_file
    NOMAD_CLIENT_KEY  = var.key_file
    NOMAD_TOKEN       = var.nomad_token
  }
}

resource "enos_local_exec" "run_tests" {
  environment = merge(
    local.nomad_env, {
      NODES_TO_DRAIN = var.nodes_to_drain
  })

  scripts = [
    abspath("${path.module}/scripts/drain.sh"),
  ]
}
