# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "foo" {
  constraint {
    attribute = "$attr.kernel.version"
    regexp    = "[0-9.]+"
  }
}
