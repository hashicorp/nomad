job "sleep" {
  datacenters = ["dc1"]

  group "sleep" {
    task "sleep" {
      driver = "docker"

      config {
        image = "busybox"
        args = ["sleep", "10000"]
      }

      resources {
        cpu    = 100
        memory = 32
      }
    }
  }
}

