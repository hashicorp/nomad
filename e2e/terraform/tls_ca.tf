resource "tls_private_key" "ca" {
  algorithm   = "ECDSA"
  ecdsa_curve = "P384"
}

resource "tls_self_signed_cert" "ca" {
  key_algorithm   = "ECDSA"
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

resource "tls_private_key" "api_client" {
  algorithm   = "ECDSA"
  ecdsa_curve = "P384"
}

resource "tls_cert_request" "api_client" {
  key_algorithm   = "ECDSA"
  private_key_pem = tls_private_key.api_client.private_key_pem

  subject {
    common_name = "${local.random_name} api client"
  }
}

resource "tls_locally_signed_cert" "api_client" {
  cert_request_pem   = tls_cert_request.api_client.cert_request_pem
  ca_key_algorithm   = tls_private_key.ca.algorithm
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

resource "local_file" "api_client_key" {
  filename = "keys/tls_api_client.key"
  content  = tls_private_key.api_client.private_key_pem
}

resource "local_file" "api_client_cert" {
  filename = "keys/tls_api_client.crt"
  content  = tls_locally_signed_cert.api_client.cert_pem
}
