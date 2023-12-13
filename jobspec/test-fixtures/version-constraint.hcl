# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


job "foo" {
  constraint {
    attribute = "$attr.kernel.version"
    version   = "~> 3.2"
  }
}
