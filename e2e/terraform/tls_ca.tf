# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# tls_ca.tf defines the certificate authority we use for mTLS

resource "tls_private_key" "ca" {
  algorithm   = "ECDSA"
  ecdsa_curve = "P384"
}

resource "tls_self_signed_cert" "ca" {
  private_key_pem = tls_private_key.ca.private_key_pem

  subject {
    common_name  = "${local.random_name} Nomad E2E Cluster"
    organization = local.random_name
  }

  validity_period_hours = 720

  is_ca_certificate = true
  allowed_uses      = ["cert_signing"]
}

resource "local_file" "ca_key" {
  filename = "keys/tls_ca.key"
  content  = tls_private_key.ca.private_key_pem
}

resource "local_file" "ca_cert" {
  filename = "keys/tls_ca.crt"
  content  = tls_self_signed_cert.ca.cert_pem
}
