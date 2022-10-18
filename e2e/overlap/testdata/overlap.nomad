job "overlap" {
  datacenters = ["dc1", "dc2"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "overlap" {
    count = 1

    task "test" {
      driver = "raw_exec"

      # Delay shutdown to delay next placement
      shutdown_delay = "10s"

      config {
        command = "bash"
        args    = ["-c", "sleep 15000"]
      }

      resources {
        cpu    = "500"
        memory = "50"
      }
    }
  }
}

