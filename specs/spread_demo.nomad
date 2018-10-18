job "redis1" {
    datacenters = ["dc1", "dc2"]
    spread {
      attribute = "${node.datacenter}"
      weight = 100
    }

  group "cache1" {
    count = 10
 
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
