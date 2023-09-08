# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

resource "local_sensitive_file" "nomad_systemd_unit_file" {
  content         = templatefile("etc/nomad.d/nomad-${var.role}.service", {})
  filename        = "${local.upload_dir}/nomad.d/nomad.service"
  file_permission = "0600"
}

resource "null_resource" "install_nomad_binary_linux" {
  count    = var.platform == "linux" ? 1 : 0
  triggers = { nomad_binary_sha = filemd5(var.nomad_local_binary) }

  connection {
    type        = "ssh"
    user        = var.connection.user
    host        = var.instance.public_ip
    port        = var.connection.port
    private_key = file(var.connection.private_key)
    timeout     = "5m"
  }

  provisioner "file" {
    source      = var.nomad_local_binary
    destination = "/tmp/nomad"
  }
  provisioner "remote-exec" {
    inline = [
      "sudo mv /tmp/nomad /usr/local/bin/nomad",
      "sudo chmod +x /usr/local/bin/nomad",
    ]
  }
}

resource "null_resource" "install_consul_configs_linux" {
  count = var.platform == "linux" ? 1 : 0

  depends_on = [
    null_resource.upload_consul_configs,
  ]

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
      "mkdir -p /etc/consul.d",
      "sudo rm -rf /etc/consul.d/*",
      "sudo mv /tmp/consul_ca.pem /etc/consul.d/ca.pem",
      "sudo mv /tmp/consul_client_acl.json /etc/consul.d/acl.json",
      "sudo mv /tmp/consul_client.json /etc/consul.d/consul_client.json",
      "sudo mv /tmp/consul_client_base.json /etc/consul.d/consul_client_base.json",
      "sudo mv /tmp/consul.service /etc/systemd/system/consul.service",
    ]
  }
}

locals {
  data_owner = var.role == "client" ? "root" : "nomad"
}

resource "null_resource" "install_nomad_configs_linux" {
  count = var.platform == "linux" ? 1 : 0

  depends_on = [
    null_resource.upload_nomad_configs,
  ]

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
      "mkdir -p /etc/nomad.d",
      "mkdir -p /opt/nomad/data",
      "sudo chmod 0700 /opt/nomad/data",
      "sudo chown ${local.data_owner}:${local.data_owner} /opt/nomad/data",
      "sudo rm -rf /etc/nomad.d/*",
      "sudo mv /tmp/consul.hcl /etc/nomad.d/consul.hcl",
      "sudo mv /tmp/vault.hcl /etc/nomad.d/vault.hcl",
      "sudo mv /tmp/base.hcl /etc/nomad.d/base.hcl",
      "sudo mv /tmp/${var.role}-${var.platform}.hcl /etc/nomad.d/${var.role}-${var.platform}.hcl",
      "sudo mv /tmp/${var.role}-${var.platform}-${var.index}.hcl /etc/nomad.d/${var.role}-${var.platform}-${var.index}.hcl",
      "sudo mv /tmp/.environment /etc/nomad.d/.environment",

      # TLS
      "sudo mkdir /etc/nomad.d/tls",
      "sudo mv /tmp/tls.hcl /etc/nomad.d/tls.hcl",
      "sudo mv /tmp/agent-${var.instance.public_ip}.key /etc/nomad.d/tls/agent.key",
      "sudo mv /tmp/agent-${var.instance.public_ip}.crt /etc/nomad.d/tls/agent.crt",
      "sudo mv /tmp/tls_proxy.key /etc/nomad.d/tls/tls_proxy.key",
      "sudo mv /tmp/tls_proxy.crt /etc/nomad.d/tls/tls_proxy.crt",
      "sudo mv /tmp/self_signed.key /etc/nomad.d/tls/self_signed.key",
      "sudo mv /tmp/self_signed.crt /etc/nomad.d/tls/self_signed.crt",
      "sudo mv /tmp/ca.crt /etc/nomad.d/tls/ca.crt",

      "sudo mv /tmp/nomad.service /etc/systemd/system/nomad.service",
    ]
  }

}

resource "null_resource" "restart_linux_services" {
  count = var.platform == "linux" ? 1 : 0

  depends_on = [
    null_resource.install_nomad_binary_linux,
    null_resource.install_consul_configs_linux,
    null_resource.install_nomad_configs_linux,
  ]

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
      "sudo systemctl enable consul",
      "sudo systemctl restart consul",
      "sudo systemctl enable nomad",
      "sudo systemctl restart nomad",
    ]
  }
}
