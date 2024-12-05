# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# consul-client.tf produces the TLS certifications and configuration files for
# the Consul agents running on the Nomad server and client nodes

# TLS certs for the Consul agents

resource "tls_private_key" "consul_agents" {
  algorithm   = "ECDSA"
  ecdsa_curve = "P384"
}

resource "tls_cert_request" "consul_agents" {
  private_key_pem = tls_private_key.consul_agents.private_key_pem

  subject {
    common_name = "${local.random_name} Consul agent"
  }
}

resource "tls_locally_signed_cert" "consul_agents" {
  cert_request_pem   = tls_cert_request.consul_agents.cert_request_pem
  ca_private_key_pem = tls_private_key.ca.private_key_pem
  ca_cert_pem        = tls_self_signed_cert.ca.cert_pem

  validity_period_hours = 720

  allowed_uses = [
    "key_encipherment",
    "digital_signature",
    "client_auth",
  ]
}

resource "local_sensitive_file" "consul_agents_key" {
  content  = tls_private_key.consul_agents.private_key_pem
  filename = "uploads/shared/consul.d/agent_cert.key.pem"
}

resource "local_sensitive_file" "consul_agents_cert" {
  content  = tls_locally_signed_cert.consul_agents.cert_pem
  filename = "uploads/shared/consul.d/agent_cert.pem"
}

# Consul tokens for the Consul agents

resource "random_uuid" "consul_agent_token" {}

resource "local_sensitive_file" "consul_agent_config_file" {
  content = templatefile("${path.module}/etc/consul.d/clients.hcl", {
    token          = "${random_uuid.consul_agent_token.result}"
    autojoin_value = "auto-join-${local.random_name}"
  })
  filename        = "uploads/shared/consul.d/clients.hcl"
  file_permission = "0600"
}

# Consul tokens for the Nomad agents

resource "random_uuid" "consul_token_for_nomad" {}

resource "local_sensitive_file" "nomad_client_config_for_consul" {
  content = templatefile("${path.module}/etc/nomad.d/client-consul.hcl", {
    token               = "${random_uuid.consul_token_for_nomad.result}"
    client_service_name = "client-${local.random_name}"
    server_service_name = "server-${local.random_name}"
  })
  filename        = "uploads/shared/nomad.d/client-consul.hcl"
  file_permission = "0600"
}

resource "local_sensitive_file" "nomad_server_config_for_consul" {
  content = templatefile("${path.module}/etc/nomad.d/server-consul.hcl", {
    token               = "${random_uuid.consul_token_for_nomad.result}"
    client_service_name = "client-${local.random_name}"
    server_service_name = "server-${local.random_name}"
  })
  filename        = "uploads/shared/nomad.d/server-consul.hcl"
  file_permission = "0600"
}
