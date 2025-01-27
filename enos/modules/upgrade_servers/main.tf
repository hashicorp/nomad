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
  ssh_user           = var.platform == "windows" ? "Administrator" : "ubuntu"
  ssh_timeout        = var.platform == "windows" ? "10m" : "5m"
}

resource "enos_file" "copy_upgraded_binary" {
  //for_each    = toset(var.servers)
  source      = var.nomad_local_upgrade_binary
  destination = var.nomad_local_binary

  transport = {
    ssh = {
      host             = var.server_addr
      private_key_path = var.ssh_key_path
      user             = local.ssh_user
    }
  }
}

resource "enos_local_exec" "run_tests" {
  environment = {
    NOMAD_ADDR        = var.nomad_addr
    NOMAD_CACERT      = var.ca_file
    NOMAD_CLIENT_CERT = var.cert_file
    NOMAD_CLIENT_KEY  = var.key_file
    NOMAD_TOKEN       = local.clean_token
  }

  inline = [
    "nomad operator snapshot save -stale true -address ${var.server_addr} ${var.server_addr}.snap",
  ]
}
/* 

resource "enos_remote_exec" "restart_linux_services" {
  depends_on = [enos_file.take_server_snapshot]

  scripts = [abspath("${path.module}/scripts/init-softhsm.sh")]

  transport = {
    ssh = {
      host = each.value.public_ip
    }
  }

   inline = [
      "sudo systemctl daemon-reload",
      "sudo systemctl restart nomad",
    ]
}

resource "enos_remote_exec" "restart_windows_services" {
  depends_on = [enos_file.copy_upgraded_binary]

  scripts = [abspath("${path.module}/scripts/init-softhsm.sh")]

  transport = {
    ssh = {
      host = each.value.public_ip
    }
  }
}

resource "null_resource" "restart_linux_services" {
  count = var.platform == "linux" ? 1 : 0

  depends_on = [enos_file.copy_upgraded_binary]

  connection {
    type        = "ssh"
    user        = var.connection.user
    host        = var.instance.public_ip
    port        = var.connection.port
    private_key = file(var.connection.private_key)
    timeout     = "5m"
  }

  provisioner "remote-exec" {
    inline = [
      "sudo systemctl daemon-reload",
      "sudo systemctl restart nomad",
    ]
  }
}

resource "null_resource" "restart_windows_services" {
  count = var.platform == "windows" ? 1 : 0

  depends_on = [enos_file.copy_upgraded_binary]

  connection {
    type            = "ssh"
    user            = var.connection.user
    host            = var.instance.public_ip
    port            = var.connection.port
    private_key     = file(var.connection.private_key)
    target_platform = "windows"
    timeout         = "10m"
  }

  provisioner "remote-exec" {
    inline = [
      "powershell Restart-Service Nomad"
    ]
  }
}
 */
