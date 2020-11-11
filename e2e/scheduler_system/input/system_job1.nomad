job "system_job" {
  datacenters = ["dc1"]

  type = "system"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "system_job_group" {
    count = 1

    restart {
      attempts = 10
      interval = "1m"

      delay = "2s"
      mode  = "delay"
    }

    task "system_task" {
      driver = "docker"

      config {
        image = "bash:latest"

        command = "bash"
        args    = ["-c", "sleep 15000"]
      }

      env {
        version = "2"
      }
    }
  }
}
