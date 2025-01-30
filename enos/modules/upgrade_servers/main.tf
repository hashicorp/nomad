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
  snap_file          = "${var.name}.snap"
}

resource "enos_local_exec" "take_server_snapshot" {
  environment = {
    NOMAD_ADDR        = var.nomad_addr
    NOMAD_CACERT      = var.ca_file
    NOMAD_CLIENT_CERT = var.cert_file
    NOMAD_CLIENT_KEY  = var.key_file
    NOMAD_TOKEN       = var.nomad_token
  }

  inline = [
    "nomad operator snapshot save ${local.snap_file}",
  ]
}

module upgrade_single_cluster {
  depends_on = [enos_local_exec.take_server_snapshot]
  
  source   = "../upgrade_single_server"
  for_each = { for i, val in var.servers : i => val }

  nomad_addr                 = var.nomad_addr
  ca_file                    = var.ca_file
  cert_file                  = var.cert_file
  key_file                   = var.key_file
  nomad_token                = var.nomad_token
  platform                   = var.platform
  server_address             = each.value
  nomad_local_upgrade_binary = var.nomad_local_upgrade_binary
  ssh_key_path               = var.ssh_key_path
}

resource "enos_local_exec" "restore_server_snapshot" {
  depends_on = [module.upgrade_single_cluster]

  environment = {
    NOMAD_ADDR        = var.nomad_addr
    NOMAD_CACERT      = var.ca_file
    NOMAD_CLIENT_CERT = var.cert_file
    NOMAD_CLIENT_KEY  = var.key_file
    NOMAD_TOKEN       = var.nomad_token
  }

  inline = [
    "nomad operator snapshot restore ${local.snap_file}",
  ]
}
