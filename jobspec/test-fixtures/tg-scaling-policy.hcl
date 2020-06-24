job "elastic" {
  group "group" {
    scaling {
      enabled = false
      min     = 5
      max     = 100

      policy {
        foo = "bar"
        b   = true
        val = 5
        f   =.1
      }
    }
  }
}
