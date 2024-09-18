# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

container {
  local_daemon = true

  secrets {
    all               = true
    skip_path_strings = ["/website/content/"]
  }

  dependencies    = true
  alpine_security = true
}

binary {
  go_modules = true
  osv        = true
  go_stdlib  = true
  nvd        = false

  secrets {
    all               = true
    skip_path_strings = ["/website/content/"]
  }
}
