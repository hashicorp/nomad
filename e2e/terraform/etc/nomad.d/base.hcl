# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

bind_addr    = "0.0.0.0"
data_dir     = "${data_dir}"
enable_debug = true
log_level    = "debug"

audit {
  enabled = true
}

acl {
  enabled = true

  # These values are used by the testACLTokenExpiration test within the acl
  # test suite. If these need to be updated, please ensure the new values are
  # reflected within the test suite and do not break the tests. Thanks.
  token_min_expiration_ttl = "1s"
  token_max_expiration_ttl = "24h"
}

telemetry {
  collection_interval        = "1s"
  disable_hostname           = true
  prometheus_metrics         = true
  publish_allocation_metrics = true
  publish_node_metrics       = true
}
