listener "tcp" {
  address = "0.0.0.0:8200"

  tls_disable                        = false
  tls_require_and_verify_client_cert = true

  tls_client_ca_file = "/etc/vault.d/tls/ca.crt"
  tls_cert_file      = "/etc/vault.d/tls/agent.crt"
  tls_key_file       = "/etc/vault.d/tls/agent.key"
}

# this autounseal key is created by Terraform in the E2E infrastructure repo
# and should be used only for these tests
seal "awskms" {
  region     = "us-east-1"
  kms_key_id = "74b7e226-c745-4ddd-9b7f-2371024ee37d"
}

storage "consul" {
  address = "127.0.0.1:8501"
  scheme  = "https"

  tls_ca_file   = "/etc/vault.d/tls/ca.crt"
  tls_cert_file = "/etc/vault.d/tls/agent.crt"
  tls_key_file  = "/etc/vault.d/tls/agent.key"
}
