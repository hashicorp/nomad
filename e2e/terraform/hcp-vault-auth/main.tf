# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# Vault cluster admin tokens expire after 6 hours, so we need to
# generate them fresh for test runs. But we can't generate the token
# and then use that token with the vault provider in the same
# Terraform run. So you'll need to apply this TF config separately
# from the root configuratiion.

variable "hcp_vault_cluster_id" {
  description = "The ID of the HCP Vault cluster"
  type        = string
  default     = "nomad-e2e-shared-hcp-vault"
}

variable "hcp_vault_namespace" {
  description = "The namespace where the HCP Vault cluster policy works"
  type        = string
  default     = "admin"
}

data "hcp_vault_cluster" "e2e_shared_vault" {
  cluster_id = var.hcp_vault_cluster_id
}

resource "hcp_vault_cluster_admin_token" "admin" {
  cluster_id = data.hcp_vault_cluster.e2e_shared_vault.cluster_id
}

output "message" {
  value = <<EOM
Your cluster admin token has been provisioned! To prepare the test runner
environment, run:

   $(terraform output --raw environment)
EOM

}

output "environment" {
  description = "get connection config by running: $(terraform output environment)"
  sensitive   = true
  value       = <<EOM
export VAULT_TOKEN=${hcp_vault_cluster_admin_token.admin.token}
export VAULT_NAMESPACE=${var.hcp_vault_namespace}
export VAULT_ADDR=${data.hcp_vault_cluster.e2e_shared_vault.vault_public_endpoint_url}

EOM

}
