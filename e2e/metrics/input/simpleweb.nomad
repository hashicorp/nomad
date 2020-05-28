job "simpleweb" {
  datacenters = ["dc1"]
  type        = "system"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "simpleweb" {
    task "simpleweb" {
      driver = "docker"

      config {
        image = "nginx:latest"

        port_map {
          http = 8080
        }
      }

      resources {
        cpu    = 256
        memory = 128

        network {
          mbits = 1
          port "http" {}
        }
      }

      // TODO(tgross): this isn't passing health checks
      service {
        port = "http"
        name = "simpleweb"
        tags = ["simpleweb"]

        check {
          type     = "tcp"
          port     = "http"
          interval = "10s"
          timeout  = "2s"
        }
      }
    }
  }
}
