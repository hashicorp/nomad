# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# tls_client.tf defines the mTLS certs that'll be used by the E2E test
# runner

resource "tls_private_key" "api_client" {
  algorithm   = "ECDSA"
  ecdsa_curve = "P384"
}

resource "tls_cert_request" "api_client" {
  private_key_pem = tls_private_key.api_client.private_key_pem

  subject {
    common_name = "${local.random_name} api client"
  }
}

resource "tls_locally_signed_cert" "api_client" {
  cert_request_pem   = tls_cert_request.api_client.cert_request_pem
  ca_private_key_pem = tls_private_key.ca.private_key_pem
  ca_cert_pem        = tls_self_signed_cert.ca.cert_pem

  validity_period_hours = 720

  # Reasonable set of uses for a server SSL certificate.
  allowed_uses = [
    "key_encipherment",
    "digital_signature",
    "client_auth",
  ]
}

resource "local_sensitive_file" "api_client_key" {
  content  = tls_private_key.api_client.private_key_pem
  filename = "keys/tls_api_client.key"
}

resource "local_sensitive_file" "api_client_cert" {
  content  = tls_locally_signed_cert.api_client.cert_pem
  filename = "keys/tls_api_client.crt"
}

# Self signed cert for reverse proxy

resource "tls_private_key" "self_signed" {
  algorithm   = "ECDSA"
  ecdsa_curve = "P384"
}

resource "tls_self_signed_cert" "self_signed" {
  private_key_pem = tls_private_key.self_signed.private_key_pem
  subject {
    common_name  = "${local.random_name}.local"
    organization = "HashiCorp, Inc."
  }

  ip_addresses = toset(aws_instance.client_ubuntu_jammy_amd64.*.public_ip)

  validity_period_hours = 720
  allowed_uses = [
    "server_auth"
  ]
}

resource "local_sensitive_file" "self_signed_key" {
  content  = tls_private_key.self_signed.private_key_pem
  filename = "keys/self_signed.key"
}

resource "local_sensitive_file" "self_signed_cert" {
  content  = tls_self_signed_cert.self_signed.cert_pem
  filename = "keys/self_signed.crt"
}
