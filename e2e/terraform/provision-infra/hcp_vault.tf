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

// Use stable naming formatting, so that e2e tests can rely on the
// CLUSTER_UNIQUE_IDENTIFIER env var to re-build these names when they need to.
//
// If these change, downstream tests will need to be updated as well, most
// notably vaultsecrets.
locals {
  workload_identity_path   = "jwt-nomad-${local.random_name}"
  workload_identity_role   = "jwt-nomad-${local.random_name}-workloads"
  workload_identity_policy = "jwt-nomad-${local.random_name}-workloads"
}

// The authentication backed is used by Nomad to generated workload identities
// for allocations.
//
// Nomad is running TLS, so we must pass the CA and HTTPS endpoint. Due to
// limitations within Vault at the moment, the Nomad TLS configuration must set
// "verify_https_client=false". Vault will return an error without this when
// writing the auth backend.
resource "vault_jwt_auth_backend" "nomad_cluster" {
  depends_on         = [null_resource.bootstrap_nomad_acls]
  default_role       = local.workload_identity_role
  jwks_url           = "https://${aws_instance.server[0].private_ip}:4646/.well-known/jwks.json"
  jwks_ca_pem        = tls_self_signed_cert.ca.cert_pem
  jwt_supported_algs = ["RS256"]
  path               = local.workload_identity_path
}

// This is our default role for the nomad JWT authentication backend within
// Vault.
resource "vault_jwt_auth_backend_role" "nomad_cluster" {
  backend                 = vault_jwt_auth_backend.nomad_cluster.path
  bound_audiences         = ["vault.io"]
  role_name               = local.workload_identity_role
  role_type               = "jwt"
  token_period            = 1800
  token_policies          = [local.workload_identity_policy]
  token_type              = "service"
  user_claim              = "/nomad_job_id"
  user_claim_json_pointer = true

  claim_mappings = {
    nomad_namespace = "nomad_namespace"
    nomad_job_id    = "nomad_job_id"
    nomad_task      = "nomad_task"
  }
}

// Enable a KV secrets backend using the generated name for the path, so that
// multiple clusters can run simultaneously and that failed destroys do not
// impact subsequent runs.
resource "vault_mount" "nomad_cluster" {
  path    = local.random_name
  type    = "kv"
  options = { version = "2" }
}

// This Vault policy is linked from default Nomad WI auth backend role and uses
// Nomad's documented default policy for workloads as an outline. It grants
// access to the KV path enabled above, making it available to all e2e tests by
// default.
resource "vault_policy" "nomad-workloads" {
  name = local.workload_identity_policy
  policy = templatefile("${path.module}/templates/vault-acl-jwt-policy-nomad-workloads.hcl.tpl", {
    AUTH_METHOD_ACCESSOR = vault_jwt_auth_backend.nomad_cluster.accessor
    MOUNT                = local.random_name
  })
}

# Nomad agent configuration for Vault
resource "local_sensitive_file" "nomad_config_for_vault" {
  content = templatefile("${path.module}/provision-nomad/etc/nomad.d/vault.hcl", {
    jwt_auth_backend_path = local.workload_identity_path
    url                   = data.hcp_vault_cluster.e2e_shared_vault.vault_private_endpoint_url
    namespace             = var.hcp_vault_namespace
  })
  filename        = "${local.uploads_dir}/shared/nomad.d/vault.hcl"
  file_permission = "0600"
}
