job "redis" {
  datacenters = ["dc1", "dc2"]

  group "cache" {
    count = 4
    update {
      max_parallel = 1
      min_healthy_time = "5s"
      healthy_deadline = "30s"
      progress_deadline = "1m"
    }
    restart {
      mode = "fail"
      attempts = 0
    }
    reschedule {
      attempts = 3
      interval = "10m"
      unlimited = false
    }
    task "redis" {
      driver = "docker"

      config {
        image = "redis:4.0"
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