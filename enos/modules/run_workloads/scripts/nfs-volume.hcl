# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

type      = "csi"
id        = "nfsTestVolume"
name      = "nfsTestVolume"
plugin_id = "rocketduck-nfs"

capability {
  access_mode     = "multi-node-multi-writer"
  attachment_mode = "file-system"
}

capability {
  access_mode     = "multi-node-single-writer"
  attachment_mode = "file-system"
}

capability {
  access_mode     = "multi-node-reader-only"
  attachment_mode = "file-system"
}

parameters {
  uid  = "1000"
  gid  = "1000"
  mode = "770"
}
