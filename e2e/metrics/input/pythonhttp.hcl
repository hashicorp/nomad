# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "pythonhttp" {
  type = "service"

  group "linux" {
    constraint {
      attribute = "${attr.kernel.name}"
      value     = "linux"
    }

    update {
      min_healthy_time = "4s"
    }

    restart {
      attempts = 0
      mode     = "fail"
    }

    network {
      mode = "host"
      port "http" {}
    }

    service {
      provider = "nomad"
      name     = "pythonhttp"
      port     = "http"
      tags     = ["svc"]
      check {
        type     = "http"
        path     = "/index.html"
        interval = "5s"
        timeout  = "2s"
      }
    }

    task "python" {
      driver = "pledge"
      config {
        command = "python3"
        args = [
          "-m", "http.server", "${NOMAD_PORT_http}",
          "--directory", "${NOMAD_TASK_DIR}",
        ]
        promises = "stdio rpath inet"
        unveil   = ["r:/etc/mime.types", "r:${NOMAD_TASK_DIR}"]
      }

      template {
        destination = "local/index.html"
        data        = <<EOH
<!doctype html>
<html>
  <title>example</title>
  <body><p>Hello, friend!</p></body>
</html>
EOH
      }

      resources {
        cpu    = 100
        memory = 32
      }
    }
  }
}

