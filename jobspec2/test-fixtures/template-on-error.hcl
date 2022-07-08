job "example" {
  group "group" {
    task "task" {
      template {
        on_error = "kill"
      }
    }
  }
}
