job "connect_native_service" {
  group "group" {
    service {
      name = "example"
      task = "task1"

      connect {
        native = true
      }
    }
  }
}
