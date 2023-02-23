job "hello-nomad" {
  group "web" {
    network {
      port "http" {
        to = 8080
      }
    }

    task "app" {
      driver = "docker"

      config {
        image          = "laoqui/hello-nomad:v1"
        ports          = ["http"]
        auth_soft_fail = true
      }

      identity {
        env  = true
        file = true
      }

      resources {
        cpu    = 500
        memory = 256
      }
    }
  }
}
