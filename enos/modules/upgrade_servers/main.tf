# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

terraform {
  required_providers {
    enos = {
      source = "registry.terraform.io/hashicorp-forge/enos"
    }
  }
}

module upgrade_single_cluster {
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
