job "example" {
  datacenters = ["dc1"]

  group "cache" {
    volume "test" {
      type   = "csi"
      source = "${volume_id}"
    }

    task "redis" {
      driver = "docker"

      config {
        image = "redis:3.2"

        port_map {
          db = 6379
        }
      }

      volume_mount {
        volume      = "test"
        destination = "/test"
      }

      resources {
        cpu    = 500
        memory = 256

        network {
          mbits = 14
          port "db" {}
        }
      }
    }
  }
}
