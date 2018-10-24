job "sysredis" {
  datacenters = ["dc1"]
  priority = 20
  type = "system"
  group "cache" {
    count = 1
    update {
       max_parallel = 1
       min_healthy_time = "10s"
       healthy_deadline = "5m"
    }

    task "redis3" {
      driver = "docker"

      config {
        image = "redis:4.0"
        port_map {
          db = 6379
        }
      }

      resources {
        cpu    = 3500
        memory = 4000
        network {
          mbits = 800
          port "db" {
            static = 8801
          }
          
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
