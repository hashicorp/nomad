job "example" {
  group "group" {
    task "task" {
      template {
        error_mode = "kill"
      }
    }
  }
}
