job "example" {
  group "group" {
    task "set-to-true" {
      render_template_on_restart = true
    }
    task "set-to-false" {
      render_template_on_restart = false
    }
    task "not-set" {
      // not setting render_template_on_restart
    }
  }
}