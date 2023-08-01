job "overlap" {
  datacenters = ["dc1"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  constraint {
    attribute = "${node.unique.id}"
    value     = "<<Must be filled in by test>>"
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
        # Must be filled in by test
        cpu    = "0"
        memory = "50"
      }
    }
  }
}

