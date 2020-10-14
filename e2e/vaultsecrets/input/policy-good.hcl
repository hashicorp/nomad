path "secrets-TESTID/data/myapp" {
  capabilities = ["read"]
}

path "pki-TESTID/issue/nomad" {
  capabilities = ["create", "update", "read"]
}
