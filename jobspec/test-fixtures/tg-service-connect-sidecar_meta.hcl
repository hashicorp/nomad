job "sidecar_disablecheck" {
  type = "service"

  group "group" {
    service {
      name = "example"

      connect {
        sidecar_service {
          meta {
            test-key = "test-value"
            test-key1 = "test-value1"
            test-key2 = "test-value2"
          }
        }
      }
    }
  }
}
