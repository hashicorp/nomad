job "group_service_proxy_expose" {
  group "group" {
    service {
      name = "example"

      connect {
        sidecar_service {
          proxy {
            expose {
              path {
                path            = "/health"
                protocol        = "http"
                local_path_port = 2222
                listener_port   = "healthcheck"
              }

              path {
                path            = "/metrics"
                protocol        = "grpc"
                local_path_port = 3000
                listener_port   = "metrics"
              }
            }
          }
        }
      }
    }
  }
}
