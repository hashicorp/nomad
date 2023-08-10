# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

client {
  enabled = true

  template {
    disable_file_sandbox = true
    function_denylist    = [""]
  }
}
