job "example" {
  group "group" {
    task "task" {
      template {
        error_on_missing_key = true
      }
    }
  }
}
