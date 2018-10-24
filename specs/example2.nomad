job "example2" {
  datacenters = ["dc1"]
  priority = 20
  group "cache2" {
    count = 1

    task "redis2" {
      driver = "docker"

      config {
        image = "redis:4.0"
        port_map {
          db = 6379
        }
      }

      resources {
        cpu    = 4000
        memory = 200
        network {
          mbits = 100
          port "db" {}
        }
      }

      service {
        name = "redis-cache"
        tags = ["global", "cache"]
        port = "db"
        check {
          name     = "alive"
          type     = "tcp"
          interval = "10s"
          timeout  = "2s"
        }
      }
    }
  }
}
