# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

client {
  memory_total_mb = 5555
}

plugin "docker" {
  config {
    allow_privileged = true
  }
}

plugin "raw_exec" {
  config {
    enabled = true
  }
}
