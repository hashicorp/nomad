job "nginx" {
  datacenters = ["dc1"]
  type = "system"

  group "simpleweb" {

    update {
      stagger          = "5s"
      max_parallel     = 1
      min_healthy_time = "10s"
      healthy_deadline = "2m"
      auto_revert      = true
    }

    task "simpleweb" {
      driver = "docker"

      config {
        image = "nginxdemos/hello"

        port_map {
          http = 8080
        }
      }

      resources {
        cpu    = 500
        memory = 128

        network {
          mbits = 1
          port "http" {
          }
        }
      }

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

