# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

schema_version = 1

project {
  header_ignore = [
    "command/asset/*.hcl",
    "command/agent/bindata_assetfs.go",
  ]
}
