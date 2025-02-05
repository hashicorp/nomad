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
  binary_destination = var.platform == "windows" ? "C:/opt/nomad.exe" : "/usr/local/bin/nomad"
  ssh_user           = var.platform == "windows" ? "Administrator" : "ubuntu"
}

resource enos_local_exec "make_source_unknown_until_apply" {
  inline = ["echo ${ var.nomad_local_upgrade_binary }"]
}

resource "enos_file" "copy_upgraded_binary" {
  source      = enos_local_exec.make_source_unknown_until_apply.stdout
  destination = local.binary_destination
  chmod       = "0755"

  transport = {
    ssh = {
      host             = var.server_address
      private_key_path = var.ssh_key_path
      user             = local.ssh_user
    }
  }
}

resource "enos_remote_exec" "restart_linux_services" {
  count      = var.platform == "linux" ? 1 : 0
  depends_on = [enos_file.copy_upgraded_binary]


  transport = {
    ssh = {
      host             = var.server_address
      private_key_path = var.ssh_key_path
      user             = local.ssh_user
    }
  }

  inline = [
    "sudo systemctl daemon-reload",
    "sudo systemctl restart nomad",
  ]
}

resource "enos_remote_exec" "restart_windows_services" {
  count      = var.platform == "windows" ? 1 : 0
  depends_on = [enos_file.copy_upgraded_binary]

  transport = {
    ssh = {
      host             = var.server_address
      private_key_path = var.ssh_key_path
      user             = local.ssh_user
    }
  }

  inline = [
    "powershell Restart-Service Nomad"
  ]
}
