job "foo" {
  task "bar" {
    driver = "docker"

    scaling "cpu" {
      enabled = true
      min     = 50
      max     = 1000

      policy {
        test = "cpu"
      }
    }

    scaling "mem" {
      enabled = false
      min     = 128
      max     = 1024

      policy {
        test = "mem"
      }
    }

  }
}

