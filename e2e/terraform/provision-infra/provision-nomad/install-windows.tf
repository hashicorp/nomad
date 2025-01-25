# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

resource "null_resource" "install_nomad_binary_windows" {
  count    = var.platform == "windows" ? 1 : 0
  triggers = { nomad_binary_sha = filemd5(var.nomad_local_binary) }

  connection {
    type            = "ssh"
    user            = var.connection.user
    host            = var.instance.public_ip
    port            = var.connection.port
    private_key     = file(var.connection.private_key)
    target_platform = "windows"
    timeout         = "10m"
  }

  provisioner "file" {
    source      = var.nomad_local_binary
    destination = "/tmp/nomad"
  }
  provisioner "remote-exec" {
    inline = [
      "powershell Move-Item -Force -Path C://tmp/nomad -Destination C:/opt/nomad.exe",
    ]
  }
}

resource "null_resource" "install_consul_configs_windows" {
  count = var.platform == "windows" ? 1 : 0

  depends_on = [
    null_resource.upload_consul_configs,
  ]

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
      "powershell Remove-Item -Force -Recurse -Path C://etc/consul.d",
      "powershell New-Item -Force -Path C:// -Name opt -ItemType directory",
      "powershell New-Item -Force -Path C://etc -Name consul.d -ItemType directory",
      "powershell Move-Item -Force -Path C://tmp/consul_ca.pem  C://Windows/System32/ca.pem",
      "powershell Move-Item -Force -Path C://tmp/consul_client_acl.json C://etc/consul.d/acl.json",
      "powershell Move-Item -Force -Path C://tmp/consul_client.json C://etc/consul.d/consul_client.json",
      "powershell Move-Item -Force -Path C://tmp/consul_client_base.json C://etc/consul.d/consul_client_base.json",
    ]
  }
}

resource "null_resource" "install_nomad_configs_windows" {
  count = var.platform == "windows" ? 1 : 0

  depends_on = [
    null_resource.upload_nomad_configs,
  ]

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
      "powershell Remove-Item -Force -Recurse -Path C://etc/nomad.d",
      "powershell New-Item -Force -Path C:// -Name opt -ItemType directory",
      "powershell New-Item -Force -Path C:// -Name etc -ItemType directory",
      "powershell New-Item -Force -Path C://etc/ -Name nomad.d -ItemType directory",
      "powershell New-Item -Force -Path C://opt/ -Name nomad -ItemType directory",
      "powershell New-Item -Force -Path C://opt/nomad -Name data -ItemType directory",
      "powershell Move-Item -Force -Path C://tmp/consul.hcl C://etc/nomad.d/consul.hcl",
      "powershell Move-Item -Force -Path C://tmp/vault.hcl C://etc/nomad.d/vault.hcl",
      "powershell Move-Item -Force -Path C://tmp/base.hcl C://etc/nomad.d/base.hcl",
      "powershell Move-Item -Force -Path C://tmp/${var.role}-${var.platform}.hcl C://etc/nomad.d/${var.role}-${var.platform}.hcl",
      "powershell Move-Item -Force -Path C://tmp/${var.role}-${var.platform}-${var.index}.hcl C://etc/nomad.d/${var.role}-${var.platform}-${var.index}.hcl",
      "powershell Move-Item -Force -Path C://tmp/.environment C://etc/nomad.d/.environment",

      # TLS
      "powershell New-Item -Force -Path C://etc/nomad.d -Name tls -ItemType directory",
      "powershell Move-Item -Force -Path C://tmp/tls.hcl C://etc/nomad.d/tls.hcl",
      "powershell Move-Item -Force -Path C://tmp/agent-${var.instance.public_ip}.key C://etc/nomad.d/tls/agent.key",
      "powershell Move-Item -Force -Path C://tmp/agent-${var.instance.public_ip}.crt C://etc/nomad.d/tls/agent.crt",
      "powershell Move-Item -Force -Path C://tmp/ca.crt C://etc/nomad.d/tls/ca.crt",
    ]
  }
}

resource "null_resource" "restart_windows_services" {
  count = var.platform == "windows" ? 1 : 0

  depends_on = [
    null_resource.install_nomad_binary_windows,
    null_resource.install_consul_configs_windows,
    null_resource.install_nomad_configs_windows,
  ]

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
      "powershell Restart-Service Consul",
      "powershell Restart-Service Nomad"
    ]
  }
}
