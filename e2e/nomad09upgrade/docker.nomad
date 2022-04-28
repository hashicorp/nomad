job "sleep" {
  datacenters = ["dc1"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

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
