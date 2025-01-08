# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

name = "no-plugin"
type = "host"

# this volume spec can be used with 'volume register' after filling in the
# host_path and node_id values below
host_path = "TODO" # absolute path of the volume that was created out-of-band
node_id   = "TODO" # ID of the node where the volume was created

capability {
  access_mode     = "single-node-writer"
  attachment_mode = "file-system"
}
