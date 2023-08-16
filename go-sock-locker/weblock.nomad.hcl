job "weblock" {
  group "g" {
    count = 2

    network {
      port "http" {}
    }
    service {
      name     = "web"
      port     = "http"
      provider = "nomad"
    }

    lock {
      #path = "nomad/jobs/weblock/g/fancy-lock" # the default for this job/group
    }
    #lock {
    #  path = "locks/the-coolest"
    #}

    task "t" {
      identity {
        env = true
      }

      driver = "docker"
      config {
        image   = "python:3-alpine"
        command = "python"
        args    = ["-m", "http.server", "--directory=local", "${NOMAD_PORT_http}"]
        ports   = ["http"]
      }

      // need extra permission to read these vars...
      template {
        destination   = "local/index.html"
        change_mode   = "signal"
        change_signal = "SIGHUP"
        data          = <<-VAR
          fancy: {{ with nomadVar "nomad/jobs/weblock/g/fancy-lock" }}{{ .alloc }}{{ end }}
          coool: {{ with nomadVar "locks/the-coolest" }}{{ .alloc }}{{ end }}
        VAR
      }

      resources {
        cpu    = 25
        memory = 50
      }
    }
  }
}
