# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

resource "tls_private_key" "nomad" {
  algorithm   = "ECDSA"
  ecdsa_curve = "P384"
}

resource "tls_cert_request" "nomad" {
  private_key_pem = tls_private_key.nomad.private_key_pem
  ip_addresses    = [var.instance.public_ip, var.instance.private_ip, "127.0.0.1"]
  dns_names       = ["${var.role}.global.nomad"]

  subject {
    common_name = "${var.role}.global.nomad"
  }
}

resource "tls_locally_signed_cert" "nomad" {
  cert_request_pem   = tls_cert_request.nomad.cert_request_pem
  ca_private_key_pem = var.tls_ca_key
  ca_cert_pem        = var.tls_ca_cert

  validity_period_hours = 720

  # Reasonable set of uses for a server SSL certificate.
  allowed_uses = [
    "key_encipherment",
    "digital_signature",
    "client_auth",
    "server_auth",
  ]
}

resource "local_sensitive_file" "nomad_client_key" {
  content  = tls_private_key.nomad.private_key_pem
  filename = "keys/agent-${var.instance.public_ip}.key"
}

resource "local_sensitive_file" "nomad_client_cert" {
  content  = tls_locally_signed_cert.nomad.cert_pem
  filename = "keys/agent-${var.instance.public_ip}.crt"
}
