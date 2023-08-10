# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "elastic" {
  group "group" {
    scaling {
      enabled = false
      min     = 5
      max     = 100

      policy {
        foo = "right"
        b   = true
      }

      policy {
        foo = "wrong"
        c   = false
      }
    }
  }
}
