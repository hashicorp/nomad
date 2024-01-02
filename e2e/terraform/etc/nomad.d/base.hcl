# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

bind_addr    = "0.0.0.0"
data_dir     = "${data_dir}"
enable_debug = true
log_level    = "debug"

audit {
  enabled = true
}

telemetry {
  collection_interval        = "1s"
  disable_hostname           = true
  prometheus_metrics         = true
  publish_allocation_metrics = true
  publish_node_metrics       = true
}
