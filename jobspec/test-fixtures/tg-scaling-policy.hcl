# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "elastic" {
  group "group" {
    scaling {
      enabled = false
      min     = 5
      max     = 100

      policy {
        foo = "bar"
        b   = true
        val = 5
        f   = 0.1

        check "foo" {
          query = "some_query"
        }
      }
    }
  }
}
