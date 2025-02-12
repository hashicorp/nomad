# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

output "nomad_binary_info_map" {
  description = "Map containing information about the Nomad binary for each of the requested OSs"
  value = {
    for os in var.oss : os => {
      path         = module.fetch_artifact[os].nomad_local_binary
      artifact_url = module.fetch_artifact[os].artifact_url
      artifact_sha = module.fetch_artifact[os].artifact_sha
    }
  }
}
