job "test1" {
  datacenters = ["dc1", "dc2"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "test1" {
    count = 1

    volume "data" {
      type   = "host"
      source = "shared_data"
    }

    task "test" {
      driver = "docker"

      volume_mount {
        volume      = "data"
        destination = "/tmp/foo"
      }

      config {
        image = "bash:latest"

        command = "bash"
        args    = ["-c", "sleep 15000"]
      }
    }
  }
}
