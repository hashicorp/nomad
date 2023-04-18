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
