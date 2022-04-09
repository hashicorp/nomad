job "deployment_auto.nomad" {
  datacenters = ["dc1", "dc2"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "one" {
    count = 3

    update {
      max_parallel     = 3
      auto_promote     = true
      canary           = 2
      min_healthy_time = "1s"
    }

    task "one" {
      driver = "raw_exec"

      env {
        version = "1"
      }

      config {
        command = "/bin/sleep"

        # change args to update the job, the only changes
        args = ["1000000"]
      }

      resources {
        cpu    = 20
        memory = 20
      }
    }
  }
}
