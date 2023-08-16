job "sidelock" {
  group "g" {
    count = 2

    task "lock" {
      lifecycle {
        hook    = "prestart"
        sidecar = true
      }
      identity {
        env = true
      }
      driver = "docker"
      config {
        image = "sock-locker:local"
      }
      resources {
        cpu    = 25
        memory = 50
      }
    }

    task "work" {
      driver = "exec"
      config {
        command = "bash"
        args    = ["local/work.sh"]
      }
      template {
        destination = "local/work.sh"
        data        = file("work.sh")
      }
      template {
        change_mode   = "signal"
        change_signal = "SIGHUP"
        destination   = "local/lock"
        # app code should check the value of this file against $NOMAD_ALLOC_ID
        data = <<-VAR
          alloc: {{ with nomadVar "nomad/jobs/sidelock/g/lock" }}{{ .alloc }}{{ end }}
          VAR
      }
      resources {
        cpu    = 25
        memory = 50
      }
    }
  }
}
