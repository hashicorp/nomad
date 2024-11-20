# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1
name         = "test"
type         = "host"
plugin_id    = "example-host-volume"
capacity_min = "50mb"
capacity_max = "50mb"

capability {
  access_mode     = "single-node-writer"
  attachment_mode = "file-system"
}

parameters {
  a = "ayy"
}

# TODO(1.10.0): don't require node_pool
node_pool = "default"
