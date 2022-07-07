job "example" {
  group "group" {
    task "task" {
      template {
        on_render_error = "kill"
      }
    }
  }
}
