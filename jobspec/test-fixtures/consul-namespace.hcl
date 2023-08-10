# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "consul-namespace" {
  group "group" {
    consul {
      namespace = "foo"
    }
  }
}
