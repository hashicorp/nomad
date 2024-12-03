# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

output "local_binary" {
  value       = var.binary_path
  description = "Path where the binary will be placed"
}

output "vault_artifactory_release" {
  description = "Binary information returned from the artifactory"
  value = {
    url    = data.enos_artifactory_item.nomad.results[0].url
    sha256 = data.enos_artifactory_item.nomad.results[0].sha256
  }
}