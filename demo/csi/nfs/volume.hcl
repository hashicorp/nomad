# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

id        = "csi-nfs"
name      = "csi-nfs"
type      = "csi"
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
  # set volume directory user/group/perms (optional)
  uid  = "1000" # vagrant
  gid  = "1000"
  mode = "770"
}
