# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "parse-ports" {
  group "group" {
    network {
      port "static" {
        static = 9000
      }

      port "dynamic" {}
    }
  }
}
