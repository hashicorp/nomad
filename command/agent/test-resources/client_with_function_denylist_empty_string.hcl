# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

client {
  enabled = true

  template {
    disable_file_sandbox = true
    function_denylist    = [""]
  }
}
