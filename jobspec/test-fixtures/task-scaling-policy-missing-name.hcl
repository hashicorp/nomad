job "foo" {
  task "bar" {
    driver = "docker"

    scaling {
      enabled = true
      min     = 50
      max     = 1000

      policy {
        test = "cpu"
      }
    }

  }
}

