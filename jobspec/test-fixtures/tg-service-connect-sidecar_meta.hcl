job "sidecar_disablecheck" {
  type = "service"

  group "group" {
    service {
      name = "example"

      connect {
        sidecar_service {
          meta {
            test-key = "test-value"
          }
        }
      }
    }
  }
}
