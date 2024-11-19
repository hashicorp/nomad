# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1
name      = "test"
type      = "host"
plugin_id = "example-host-volume"
#plugin_id    = "mkdir"
capacity_min = "50mb"
capacity_max = "50mb"
parameters {
  a = "ayy"
}

# TODO(1.10): don't require node_pool
node_pool = "default"
