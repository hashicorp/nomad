job "redis" {
  datacenters = ["dc1"]

  type = "system"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "cache" {
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
        version = "1"
      }

      logs {
        max_files     = 1
        max_file_size = 9
      }

      resources {
        cpu = 20 # 500 MHz    

        memory = 40 # 256MB

        network {
          mbits = 1
          port  "db"  {}
        }
      }
    }
  }
}
