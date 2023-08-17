# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

container {
  secrets {
    all = false
  }

  dependencies    = false
  alpine_security = false
}

binary {
  go_modules = true
  osv        = false
  nvd        = false

  secrets {
    all = true
  }
}
