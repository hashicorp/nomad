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
  clean_token        = trimspace(var.nomad_token) # Somewhere in the process, a newline is added to the token.
  binary_destination = var.platform == "windows" ? "C:/opt/nomad.exe" : "/usr/local/bin/nomad"
  ssh_user           = var.platform == "windows" ? "Administrator" : "ubuntu"
  tmp                = replace(var.server_address, ".", "_")
  snap_file          = "server-${local.tmp}.snap"
}

resource "enos_file" "copy_upgraded_binary" {
  source      = var.nomad_local_upgrade_binary
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

resource "enos_local_exec" "take_server_snapshot" {
  environment = {
    NOMAD_ADDR        = var.nomad_addr
    NOMAD_CACERT      = var.ca_file
    NOMAD_CLIENT_CERT = var.cert_file
    NOMAD_CLIENT_KEY  = var.key_file
    NOMAD_TOKEN       = local.clean_token
  }

  inline = [
    "nomad operator snapshot save -stale=true ${local.snap_file}",
  ]
}

resource "enos_remote_exec" "restart_linux_services" {
  count      = var.platform == "linux" ? 1 : 0
  depends_on = [enos_local_exec.take_server_snapshot, enos_file.copy_upgraded_binary]


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
  depends_on = [enos_local_exec.take_server_snapshot, enos_file.copy_upgraded_binary]

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

resource "enos_local_exec" "restore_server_snapshot" {
  depends_on = [enos_remote_exec.restart_windows_services, enos_remote_exec.restart_linux_services]

  environment = {
    NOMAD_ADDR        = var.nomad_addr
    NOMAD_CACERT      = var.ca_file
    NOMAD_CLIENT_CERT = var.cert_file
    NOMAD_CLIENT_KEY  = var.key_file
    NOMAD_TOKEN       = local.clean_token
  }

  inline = [
    "nomad operator snapshot restore ${local.snap_file}",
  ]
}

