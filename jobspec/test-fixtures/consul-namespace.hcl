# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


job "consul-namespace" {
  group "group" {
    consul {
      namespace = "foo"
    }
  }
}
