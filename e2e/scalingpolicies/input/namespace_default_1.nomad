job "horizontally_scalable" {
  datacenters = ["dc1"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "horizontally_scalable" {

    scaling {
      min     = 1
      max     = 10
      enabled = true

      policy {
        // Setting a single value allows us to check the policy block is
        // handled opaquely by Nomad.
        cooldown = "13m"
      }
    }

    task "test" {
      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["-c", "sleep 15000"]
      }
    }
  }
}
