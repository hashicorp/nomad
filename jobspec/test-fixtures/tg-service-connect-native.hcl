job "connect_native_service" {
  group "group" {
    service {
      name = "example"

      connect {
        native = "foo"
      }
    }
  }
}
