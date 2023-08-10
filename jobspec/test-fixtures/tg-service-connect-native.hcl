# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "connect_native_service" {
  group "group" {
    service {
      name = "example"
      task = "task1"

      connect {
        native = true
      }
    }
  }
}
