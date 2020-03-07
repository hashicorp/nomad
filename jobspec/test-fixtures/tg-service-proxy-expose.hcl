job "group_service_proxy_expose" {
  group "group" {
    service {
      name = "example"
      connect {
        sidecar_service {
          proxy {
            expose {
              paths = [{
                path = "/health"
                protocol = "http"
                local_path_port = 2222
                listener_port = "healthcheck"
              }]
            }
          }
        }
      }
    }
  }
}
