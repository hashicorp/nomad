job "spread-system-2" {
  datacenters = ["dc1"]

  type = "system"

  group "cache" {
    // count = 2

    max_client_disconnect = "20m"
    
    // spread {
    //   attribute = "${node.datacenter}"
    // }

    network {
      port "db" {
        to = 6379
      }
    }

    task "redis" {
      driver = "docker"

      config {
        image = "redis:3.2"

        ports = ["db"]
      }

      resources {
        cpu    = 500
        memory = 256
      }
    }
  }
}
