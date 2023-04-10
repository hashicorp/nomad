# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

server {
  enabled          = true
  bootstrap_expect = 3
}

consul {
  address = "1.2.3.4:8500"
}
