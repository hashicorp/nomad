# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

locals {

  // file name extensions for the install packages of Nomad for the various 
  // architectures, distributions and editions
  package_extensions = {
    amd64 = {
      linux   = "_linux_amd64.zip"
      windows = "_windows_amd64.zip"
    }

    arm64 = {
      linux   = "_linux_arm64.zip"
    }
  }

  // product_version --> artifact_version
  artifact_version = replace(var.product_version, var.edition, "ent")

  # Prefix for the artifact name. Ex: nomad_ and nomad-enterprise_
  artifact_name_prefix = var.artifact_type == "package" ? local.artifact_package_release_names[var.distro][var.edition] : "nomad_"
  
  # Suffix and extension for the artifact name. Ex: _linux_<arch>.zip,
  artifact_name_extension = var.artifact_type == "package" ? local.package_extensions[var.arch][var.distro] : "_linux_${var.arch}.zip"
  
  # Combine prefix/suffix/extension together to form the artifact name
  artifact_name = var.artifact_type == "package" ? "${local.artifact_name_prefix}${replace(local.artifact_version, "-", "~")}${local.artifact_name_extension}" : "${local.artifact_name_prefix}${var.product_version}${local.artifact_name_extension}"
}