listener "tcp" {
  address     = "0.0.0.0:8200"
  tls_disable = 1
}

# this autounseal key is created by Terraform in the E2E infrastructure repo
# and should be used only for these tests
seal "awskms" {
  region     = "us-east-1"
  kms_key_id = "74b7e226-c745-4ddd-9b7f-2371024ee37d"
}

# Vault 1.5.4 doesn't have autodiscovery for retry_join on its
# integrated storage yet so we'll just use consul for storage
storage "consul" {}
