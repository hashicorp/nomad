# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

schema_version = 1

project {
  license        = "BUSL-1.1"
  copyright_year = 2024

  header_ignore = [
    "command/asset/*.hcl",
    "command/agent/bindata_assetfs.go",
    "ui/node_modules",

    // Enterprise files do not fall under the open source licensing. CE-ENT
    // merge conflicts might happen here, please be sure to put new CE
    // exceptions above this comment.
  ]
}
