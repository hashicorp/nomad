# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

output "nomad_local_binary" {
  description = "Path where the binary will be placed"
  value       = var.os == "windows" ? "${var.binary_path}/nomad.exe" : "${var.binary_path}/nomad"
}
