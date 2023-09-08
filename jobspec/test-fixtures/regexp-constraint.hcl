# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "foo" {
  constraint {
    attribute = "$attr.kernel.version"
    regexp    = "[0-9.]+"
  }
}
