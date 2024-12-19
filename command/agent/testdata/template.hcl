# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

client {
  template {
    function_denylist    = ["plugin"]
    disable_file_sandbox = true
    max_stale            = "7600h"

    wait {
      min = "10s"
      max = "10m"
    }

    wait_bounds {
      min = "1s"
      max = "10h"
    }

    block_query_wait = "10m"

    consul_retry {
      attempts    = 6
      backoff     = "550ms"
      max_backoff = "10m"
    }

    vault_retry {
      attempts    = 6
      backoff     = "550ms"
      max_backoff = "10m"
    }

    nomad_retry {
      attempts    = 6
      backoff     = "550ms"
      max_backoff = "10m"
    }
  }
}
