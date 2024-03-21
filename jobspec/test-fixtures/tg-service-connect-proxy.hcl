# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "service-connect-proxy" {
  type = "service"

  group "group" {
    service {
      name = "example"

      connect {
        sidecar_service {
          proxy {
            local_service_port    = 8080
            local_service_address = "10.0.1.2"

            upstreams {
              destination_name = "upstream1"
              local_bind_port  = 2001
            }

            upstreams {
              destination_name = "upstream2"
              local_bind_port  = 2002
            }

            expose {
              path {
                path            = "/metrics"
                protocol        = "http"
                local_path_port = 9001
                listener_port   = "metrics"
              }

              path {
                path            = "/health"
                protocol        = "http"
                local_path_port = 9002
                listener_port   = "health"
              }
            }

            transparent_proxy {
              uid                    = "101"
              outbound_port          = 15001
              exclude_inbound_ports  = ["www", "9000"]
              exclude_outbound_ports = [443, 80]
              exclude_outbound_cidrs = ["10.0.0.0/8"]
              exclude_uids           = ["10", "1001"]
              no_dns                 = true
            }

            config {
              foo = "bar"
            }
          }
        }
      }
    }
  }
}
