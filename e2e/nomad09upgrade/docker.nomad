job "sleep" {
  datacenters = ["dc1"]

  group "sleep" {
    task "sleep" {
      driver = "docker"

      config {
        image = "redis"
      }

      resources {
        cpu    = 100
        memory = 32
      }
    }
  }
}

