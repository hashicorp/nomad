# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

output "servers" {
  value = module.provision-infra.servers
}

output "linux_clients" {
  value = module.provision-infra.linux_clients
}

output "windows_clients" {
  value = module.provision-infra.windows_clients
}

output "message" {
  value = module.provision-infra.message
}

# Note: Consul and Vault environment needs to be set in test
# environment before the Terraform run, so we don't have that output
# here
output "environment" {
  description = "get connection config by running: $(terraform output environment)"
  sensitive   = true
  value       = module.provision-infra.environment
}