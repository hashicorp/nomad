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
  clean_token = trimspace(var.nomad_token) #Somewhere in the process, a newline is added to teh token.
}

resource "enos_local_exec" "run_tests" {
  environment = {
    NOMAD_ADDR        = var.nomad_addr
    NOMAD_CACERT      = var.ca_file
    NOMAD_CLIENT_CERT = var.cert_file
    NOMAD_CLIENT_KEY  = var.key_file
    NOMAD_TOKEN       = local.clean_token
    SERVERS           = var.server_count
    CLIENTS           = var.client_count
    JOBS              = var.jobs_count
    ALLOCS            = var.alloc_count
  }

  scripts = [
    abspath("${path.module}/scripts/servers.sh"),
    abspath("${path.module}/scripts/clients.sh"),
    abspath("${path.module}/scripts/jobs.sh"),
    abspath("${path.module}/scripts/allocs.sh")
  ]
}
