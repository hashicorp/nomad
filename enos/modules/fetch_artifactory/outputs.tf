# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

output "vault_artifactory_release" {
  description = "Binary information returned from the artifactory"

  value = {
    url    = data.enos_artifactory_item.nomad.results[0].url
    sha256 = data.enos_artifactory_item.nomad.results[0].sha256
  }
}

output "nomad_local_binary" {
  description = "Path where the binary will be placed"
  value       = var.os == "windows" ? "${var.binary_path}/nomad.exe" : "${var.binary_path}/nomad"
}
