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
  servers_addr = join(" ", var.servers)
}

resource "enos_local_exec" "run_tests" {
  environment = {
    NOMAD_ADDR        = var.nomad_addr
    NOMAD_CACERT      = var.ca_file
    NOMAD_CLIENT_CERT = var.cert_file
    NOMAD_CLIENT_KEY  = var.key_file
    NOMAD_TOKEN       = var.nomad_token
    SERVER_COUNT      = var.server_count
    CLIENT_COUNT      = var.client_count
    JOB_COUNT         = var.jobs_count
    ALLOC_COUNT       = var.alloc_count
    SERVERS           = local.servers_addr
  }

  scripts = [
    abspath("${path.module}/scripts/servers.sh"),
    abspath("${path.module}/scripts/clients.sh"),
    abspath("${path.module}/scripts/jobs.sh"),
    abspath("${path.module}/scripts/allocs.sh")
  ]
}

 resource "enos_local_exec" "verify_versions" {
  environment = {
    NOMAD_ADDR        = var.nomad_addr
    NOMAD_CACERT      = var.ca_file
    NOMAD_CLIENT_CERT = var.cert_file
    NOMAD_CLIENT_KEY  = var.key_file
    NOMAD_TOKEN       = var.nomad_token
    SERVERS_VERSION   = var.servers_version
    CLIENTS_VERSION   = var.clients_version
  }

  scripts = [
    abspath("${path.module}/scripts/versions.sh"),
  ]
} 

