# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


job "foo" {
  constraint {
    distinct_property = "${meta.rack}"
  }
}
