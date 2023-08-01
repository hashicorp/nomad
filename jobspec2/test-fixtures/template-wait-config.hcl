job "example" {
  group "group" {
    task "task" {
      template {
        wait {
          min = "5s"
          max = "60s"
        }
      }
    }
  }
}
