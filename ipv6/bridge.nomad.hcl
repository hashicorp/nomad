job "bridge" {
  group "g" {
    network {
      mode = "host"
      #mode = "bridge"
      port "http" {
        static = 8000
        to     = 8000
      }
    }
    service {
      name     = "web"
      port     = "http"
      provider = "nomad"
    }
    task "t" {
      driver = "docker"
      config {
        image   = "python:slim"
        command = "bash"
        args    = ["-c", "echo hi > local/index.html ; python3 -m http.server -d local/"]
        ports   = ["http"]
        advertise_ipv6_address = true
      }
    }
    update {
      min_healthy_time = "1s"
    }
  }
}
