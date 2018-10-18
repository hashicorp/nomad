job "redis1" {
    datacenters = ["dc1", "dc2"]
    type = "service"

  group "redis1" {
    count = 5
    task "redis" {
      driver = "docker"
      
      config {
        image = "redis:3.2"
        port_map {
          db = 6379
        }
      }

      resources {
        cpu    = 500
        memory = 256
        network {
          mbits = 10
          port "db" {}
        }
      }

      service {
        name = "redis-cache1"
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
