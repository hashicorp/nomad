job "deployment_auto.nomad" {
  datacenters = ["dc1"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "one" {
    count = 3

    update {
      max_parallel = 3
      auto_promote = true
      canary       = 2
    }

    task "one" {
      driver = "raw_exec"

      env {
        version = "0"
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
