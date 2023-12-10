# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

schema_version = 1

project {
  header_ignore = [
    "command/asset/*.hcl",
    "command/agent/bindata_assetfs.go",
    "ui/node_modules",

    // Enterprise files do not fall under the open source licensing. OSS-ENT
    // merge conflicts might happen here, please be sure to put new OSS
    // exceptions above this comment.
  ]
}
