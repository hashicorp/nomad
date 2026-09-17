job "failed-logs" {
  datacenters = ["dc1"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "logger" {
    count = 1

    restart {
      attempts = 0
      mode     = "fail"
    }

    ephemeral_disk {
      size = 300
    }

    task "logger1" {
      driver = "docker"

      config {
        image   = "busybox:1"
        command = "/bin/sh"
        args    = ["-c", "echo Hello logger1; sleep 1"]
      }

      resources {
        cpu    = 100
        memory = 100
      }
    }

    task "logger2" {
      driver = "docker"

      config {
        image   = "busybox:1"
        command = "/bin/sh"
        args    = ["-c", "echo Hello logger2; sleep 1"]
      }

      resources {
        cpu    = 100
        memory = 100
      }
    }
  }
}
