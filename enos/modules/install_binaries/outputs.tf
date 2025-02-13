# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

output "binary_path" {
  description = "Binary's local path per requested OS"
  value       = { for os, res in module.fetch_artifact : os => res.nomad_local_binary }
}

output "artifact_url" {
  description = "Binary's artifactory URL per requested OS"
  value       = { for os, res in module.fetch_artifact : os => res.artifact_url }
}

output "artifact_sha" {
  description = "Binary's artifactory sha per requested OS"
  value       = { for os, res in module.fetch_artifact : os => res.artifact_sha }
}
