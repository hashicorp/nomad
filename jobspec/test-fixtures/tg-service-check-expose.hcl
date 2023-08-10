# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "group_service_proxy_expose" {
  group "group" {
    service {
      name = "example"

      connect {
        sidecar_service {
          proxy {}
        }
      }

      check {
        name   = "example-check1"
        expose = true
      }

      check {
        name   = "example-check2"
        expose = false
      }
    }
  }
}
