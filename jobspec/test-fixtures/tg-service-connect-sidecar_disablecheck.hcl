job "sidecar_disablecheck" {
  type = "service"

  group "group" {
    service {
      name = "example"

      connect {
        sidecar_service {
          disable_default_tcp_check = true
        }
      }
    }
  }
}
