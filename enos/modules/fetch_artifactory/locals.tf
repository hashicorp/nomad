# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

locals {

  path = var.edition == "ce" ? "nomad/*" : "nomad-enterprise/*"

  artifact_version = var.edition == "ce" ? "${var.product_version}" : "${var.product_version}+ent"

  package_extensions = {
    amd64 = {
      linux   = "_linux_amd64.zip"
      windows = "_windows_amd64.zip"
    }

    arm64 = {
      linux   = "_linux_arm64.zip"
    }
  } 

  artifact_name = "nomad_${local.artifact_version}${local.package_extensions[var.arch][var.os]}"
}