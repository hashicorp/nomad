job "sleep" {
  datacenters = ["dc1"]

  group "sleep" {
    task "sleep" {
      driver = "docker"

      config {
        image = "redis:5.0"
      }

      resources {
        cpu    = 100
        memory = 32
      }
    }
  }
}

