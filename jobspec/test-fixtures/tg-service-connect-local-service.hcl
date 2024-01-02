# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


job "connect-proxy-local-service" {
  type = "service"

  group "group" {
    service {
      name = "example"

      connect {
        sidecar_service {
          proxy {
            local_service_port    = 9876
            local_service_address = "10.0.1.2"
          }
        }
      }
    }
  }
}
