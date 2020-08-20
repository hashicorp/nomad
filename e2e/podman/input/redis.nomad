job "podman-redis" {
  datacenters = ["dc1"]
  type        = "service"

  group "redis" {
    task "redis" {
      driver = "podman"

      config {
        image = "docker://redis"

        port_map {
          redis = 6379
        }
      }

      resources {
        cpu    = 500
        memory = 256

        network {
          mbits = 20
          port "redis" {}
        }
      }
    }
  }
}
