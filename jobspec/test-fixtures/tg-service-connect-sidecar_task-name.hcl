job "sidecar_task_name" {
  type = "service"

  group "group" {
    service {
      name = "example"

      connect {
        sidecar_service = {}

        sidecar_task {
          name = "my-sidecar"
        }
      }
    }
  }
}
