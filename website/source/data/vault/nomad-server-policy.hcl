# Allow creating tokens under the role
path "auth/token/create/nomad-server" {
  capabilities = ["create", "update"]
}

# Allow looking up the role
path "auth/token/roles/nomad-server" {
  capabilities = ["read"]
}

# Allow looking up incoming tokens to validate they have permissions to
# access the tokens they are requesting
path "auth/token/lookup/*" {
  capabilities = ["read"]
}

# Allow revoking tokens that should no longer exist
path "/auth/token/revoke-accessor/*" {
  capabilities = ["update"]
}
