# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# Note: the test environment must have the following values set:
# export HCP_CLIENT_ID=
# export HCP_CLIENT_SECRET=
# export VAULT_TOKEN=
# export VAULT_ADDR=

data "hcp_vault_cluster" "e2e_shared_vault" {
  cluster_id = var.hcp_vault_cluster_id
}

# Vault policy for the Nomad cluster, which allows it to mint derived tokens for
# tasks. It's interpolated with the random cluster name to avoid collisions
# between concurrent E2E clusters
resource "vault_policy" "nomad" {
  name = "${local.random_name}-nomad-server"
  policy = templatefile("${path.module}/provision-nomad/etc/acls/vault/nomad-policy.hcl", {
    role = "nomad-tasks-${local.random_name}"
  })
}

resource "vault_token" "nomad" {
  policies  = [vault_policy.nomad.name]
  no_parent = true
  renewable = true
  ttl       = "72h"
}

# The default role that Nomad will use for derived tokens. It's not allowed
# access to nomad-policy so that it can only mint tokens for tasks, not for new
# clusters
resource "vault_token_auth_backend_role" "nomad_cluster" {
  role_name           = "nomad-tasks-${local.random_name}"
  disallowed_policies = [vault_policy.nomad.name]
  orphan              = true
  token_period        = "259200"
  renewable           = true
  token_max_ttl       = "0"
}

# Nomad agent configuration for Vault
resource "local_sensitive_file" "nomad_config_for_vault" {
  content = templatefile("${path.module}/provision-nomad/etc/nomad.d/vault.hcl", {
    token     = vault_token.nomad.client_token
    url       = data.hcp_vault_cluster.e2e_shared_vault.vault_private_endpoint_url
    namespace = var.hcp_vault_namespace
    role      = "nomad-tasks-${local.random_name}"
  })
  filename        = "uploads/shared/nomad.d/vault.hcl"
  file_permission = "0600"
}
