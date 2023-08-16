# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "connect_gateway_mesh" {
  group "group" {
    service {
      name = "mesh-gateway-service"

      connect {
        gateway {
          proxy {
            config {
              foo = "bar"
            }
          }

          mesh {}
        }
      }
    }
  }
}
