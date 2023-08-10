# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "foo" {
  constraint {
    attribute    = "$meta.data"
    set_contains = "foo,bar,baz"
  }
}
