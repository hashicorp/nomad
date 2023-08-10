# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

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
