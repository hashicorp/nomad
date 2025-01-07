# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

name = "no-plugin"
type = "host"

# this volume spec can be used with 'volume register' after filling in the
# host_path and node_id values below
host_path = "TODO"
node_id   = "TODO"

capability {
  access_mode     = "single-node-writer"
  attachment_mode = "file-system"
}
