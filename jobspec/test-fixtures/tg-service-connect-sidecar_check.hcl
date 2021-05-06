job "sidecar_task_name" {
  type = "service"

  group "group" {
    service {
      name = "example"

      connect {
        sidecar_service {
          check {
            type     = "tcp"
            interval = "10s"
            timeout  = "2s"
          }
        }
      }
    }
  }
}
