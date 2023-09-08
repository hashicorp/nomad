# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "sidecar_task_resources" {
  type = "service"

  group "group" {
    service {
      name = "example"

      connect {
        # should still work without sidecar_service being set (i.e. connect gateway)
        sidecar_task {
          resources {
            cpu        = 111
            memory     = 222
            memory_max = 333
          }
        }
      }
    }
  }
}
