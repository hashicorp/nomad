job "elastic" {
  group "group" {
    scaling {
      type    = "vertical_cpu"
      enabled = false
      min     = 5
      max     = 100

      policy {
        foo = "bar"
        b   = true
        val = 5
        f   = 0.1
      }
    }
  }
}
