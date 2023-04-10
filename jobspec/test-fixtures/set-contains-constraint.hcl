# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "foo" {
  constraint {
    attribute    = "$meta.data"
    set_contains = "foo,bar,baz"
  }
}
