job "elastic" {
  group "group" {
    scaling {
      enabled = false
      policy {
        foo = "bar"
        b = true
        val = 5
        f = .1
      }
    }
  }
}
