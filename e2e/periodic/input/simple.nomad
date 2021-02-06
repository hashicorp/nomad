job "periodic" {
  datacenters = ["dc1"]
  type        = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }


  periodic {
    cron             = "* * * * *"
    prohibit_overlap = true
  }

  group "group" {
    task "task" {
      driver = "docker"

      config {
        image   = "busybox:1"
        command = "/bin/sh"
        args    = ["-c", "sleep 5"]
      }
    }
  }
}

