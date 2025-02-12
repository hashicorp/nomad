# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

output "binary_path_per_os" {
  description = "Map containing information about the Nomad binary for each of the requested OSs"
  value = {
    for os in var.oss : os => module.fetch_artifact[os].nomad_local_binary
  }
}

output "artifact_url_per_os" {
  description = "Map containing information about the Nomad binary for each of the requested OSs"
  value = {
    for os in var.oss : os => module.fetch_artifact[os].artifact_sha
  }
}

output "artifact_sha_per_os" {
  description = "Map containing information about the Nomad binary for each of the requested OSs"
  value = {
    for os in var.oss : os => module.fetch_artifact[os].artifact_sha
  }
}
