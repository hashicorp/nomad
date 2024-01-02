# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


job "sidecar_task_name" {
  type = "service"

  group "group" {
    service {
      name = "example"

      connect {
        sidecar_service {}

        sidecar_task {
          name = "my-sidecar"
        }
      }
    }
  }
}
