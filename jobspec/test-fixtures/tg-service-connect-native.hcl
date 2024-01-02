# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


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
