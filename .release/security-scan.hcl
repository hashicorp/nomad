# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

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
