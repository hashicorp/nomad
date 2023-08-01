job "service_meta" {
  type = "service"

  group "group" {
    task "task" {
      service {
        name = "http-service"

        meta {
          foo = "bar"
        }
      }
    }
  }
}
