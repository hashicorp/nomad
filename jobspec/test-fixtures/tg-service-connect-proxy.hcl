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

            config {
              foo = "bar"
            }
          }
        }
      }
    }
  }
}
