# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# consul-servers.tf produces the TLS certifications and configuration files for
# the single-node Consul server cluster

# Consul token for bootstrapping the Consul server

resource "random_uuid" "consul_initial_management_token" {}

resource "local_sensitive_file" "consul_initial_management_token" {
  content         = random_uuid.consul_initial_management_token.result
  filename        = "keys/consul_initial_management_token"
  file_permission = "0600"
}

resource "local_sensitive_file" "consul_server_config_file" {
  content = templatefile("${path.module}/etc/consul.d/servers.hcl", {
    management_token = "${random_uuid.consul_initial_management_token.result}"
    token            = "${random_uuid.consul_agent_token.result}"
    nomad_token      = "${random_uuid.consul_token_for_nomad.result}"
    autojoin_value   = "auto-join-${local.random_name}"
  })
  filename        = "uploads/shared/consul.d/servers.hcl"
  file_permission = "0600"
}

# TLS cert for the Consul server

resource "tls_private_key" "consul_server" {
  algorithm   = "ECDSA"
  ecdsa_curve = "P384"
}

resource "tls_cert_request" "consul_server" {
  private_key_pem = tls_private_key.consul_server.private_key_pem
  ip_addresses    = [aws_instance.consul_server.public_ip, aws_instance.consul_server.private_ip, "127.0.0.1"]
  dns_names       = ["server.consul.global"]

  subject {
    common_name = "${local.random_name} Consul server"
  }
}

resource "tls_locally_signed_cert" "consul_server" {
  cert_request_pem   = tls_cert_request.consul_server.cert_request_pem
  ca_private_key_pem = tls_private_key.ca.private_key_pem
  ca_cert_pem        = tls_self_signed_cert.ca.cert_pem

  validity_period_hours = 720

  allowed_uses = [
    "key_encipherment",
    "digital_signature",
    "client_auth",
    "server_auth",
  ]
}

resource "local_sensitive_file" "consul_server_key" {
  content  = tls_private_key.consul_server.private_key_pem
  filename = "uploads/shared/consul.d/server_cert.key.pem"
}

resource "local_sensitive_file" "consul_server_cert" {
  content  = tls_locally_signed_cert.consul_server.cert_pem
  filename = "uploads/shared/consul.d/server_cert.pem"
}

# if consul_license is unset, it'll be a harmless empty license file
resource "local_sensitive_file" "consul_environment" {
  content = templatefile("${path.module}/etc/consul.d/.environment", {
    license = var.consul_license
  })
  filename        = "uploads/shared/consul.d/.environment"
  file_permission = "0600"
}

resource "null_resource" "upload_consul_server_configs" {

  depends_on = [
    local_sensitive_file.ca_cert,
    local_sensitive_file.consul_server_config_file,
    local_sensitive_file.consul_server_key,
    local_sensitive_file.consul_server_cert,
    local_sensitive_file.consul_environment,
  ]

  connection {
    type            = "ssh"
    user            = "ubuntu"
    host            = aws_instance.consul_server.public_ip
    port            = 22
    private_key     = file("${path.root}/keys/${local.random_name}.pem")
    target_platform = "unix"
    timeout         = "15m"
  }

  provisioner "file" {
    source      = "keys/tls_ca.crt"
    destination = "/tmp/consul_ca.pem"
  }
  provisioner "file" {
    source      = "uploads/shared/consul.d/.environment"
    destination = "/tmp/.consul_environment"
  }
  provisioner "file" {
    source      = "uploads/shared/consul.d/server_cert.pem"
    destination = "/tmp/consul_cert.pem"
  }
  provisioner "file" {
    source      = "uploads/shared/consul.d/server_cert.key.pem"
    destination = "/tmp/consul_cert.key.pem"
  }
  provisioner "file" {
    source      = "uploads/shared/consul.d/servers.hcl"
    destination = "/tmp/consul_server.hcl"
  }
  provisioner "file" {
    source      = "${path.module}/etc/consul.d/consul-server.service"
    destination = "/tmp/consul.service"
  }
}

resource "null_resource" "install_consul_server_configs" {

  depends_on = [
    null_resource.upload_consul_server_configs,
  ]

  connection {
    type            = "ssh"
    user            = "ubuntu"
    host            = aws_instance.consul_server.public_ip
    port            = 22
    private_key     = file("${path.root}/keys/${local.random_name}.pem")
    target_platform = "unix"
    timeout         = "15m"
  }

  provisioner "remote-exec" {
    inline = [
      "sudo rm -rf /etc/consul.d/*",
      "sudo mkdir -p /etc/consul.d/bootstrap",
      "sudo mv /tmp/consul_ca.pem /etc/consul.d/ca.pem",
      "sudo mv /tmp/consul_cert.pem /etc/consul.d/cert.pem",
      "sudo mv /tmp/consul_cert.key.pem /etc/consul.d/cert.key.pem",
      "sudo mv /tmp/consul_server.hcl /etc/consul.d/consul.hcl",
      "sudo mv /tmp/consul.service /etc/systemd/system/consul.service",
      "sudo mv /tmp/.consul_environment /etc/consul.d/.environment",
      "sudo systemctl daemon-reload",
      "sudo systemctl enable consul",
      "sudo systemctl restart consul",
    ]
  }
}

# Bootstrapping Consul ACLs:
#
# We can't both bootstrap the ACLs and use the Consul TF provider's
# resource.consul_acl_token in the same Terraform run, because there's no way to
# get the management token into the provider's environment after we bootstrap,
# and we want to pass various tokens in the Nomad and Consul configuration
# files. So we run a bootstrapping script that uses tokens we generate randomly.
resource "null_resource" "bootstrap_consul_acls" {
  depends_on = [null_resource.install_consul_server_configs]

  provisioner "local-exec" {
    command = "./scripts/bootstrap-consul.sh"
    environment = {
      CONSUL_HTTP_ADDR           = "https://${aws_instance.consul_server.public_ip}:8501"
      CONSUL_CACERT              = "keys/tls_ca.crt"
      CONSUL_HTTP_TOKEN          = "${random_uuid.consul_initial_management_token.result}"
      CONSUL_AGENT_TOKEN         = "${random_uuid.consul_agent_token.result}"
      NOMAD_CLUSTER_CONSUL_TOKEN = "${random_uuid.consul_token_for_nomad.result}"
    }
  }
}
