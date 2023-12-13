# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


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
