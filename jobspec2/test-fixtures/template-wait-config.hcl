job "example" {
  group "group" {
    task "task" {
      template {
        wait {
          enabled = true
          min     = "5s"
          max     = "60s"
        }
      }
    }
  }
}
