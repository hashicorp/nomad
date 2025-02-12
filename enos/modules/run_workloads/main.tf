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
  nomad_env = { NOMAD_ADDR = var.nomad_addr
    NOMAD_CACERT      = var.ca_file
    NOMAD_CLIENT_CERT = var.cert_file
    NOMAD_CLIENT_KEY  = var.key_file
  NOMAD_TOKEN = var.nomad_token }
}

resource "enos_local_exec" "wait_for_nomad_api" {
  environment = local.nomad_env

  scripts = [abspath("${path.module}/scripts/wait_for_nomad_api.sh")]
}

resource "enos_local_exec" "workloads" {
  for_each = var.workloads

  environment = local.nomad_env

  inline = ["nomad job run -var alloc_count=${each.value.alloc_count} ${path.module}/${each.value.job_spec}"]
}
