# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# This job serves the NOMAD_TASK_DIR over http.

job "http" {
  type = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "backend" {
    network {
      mode = "bridge"
      port "http" {
        to = "9999"
      }
    }

    task "http" {
      driver = "exec2"

      service {
        name     = "python-http"
        port     = "http"
        provider = "nomad"
        check {
          name     = "hi"
          type     = "http"
          path     = "/"
          interval = "3s"
          timeout  = "1s"
        }
      }

      config {
        command = "python3"
        args    = ["-m", "http.server", "9999", "--directory", "${NOMAD_TASK_DIR}"]
      }

      template {
        destination = "local/hi.html"
        data        = <<EOH
      <!doctype html>
      <html>
        <title>example</title>
        <body><p>Hello, friend!</p></body>
      </html>
      EOH
      }

      resources {
        cpu    = 500
        memory = 256
      }
    }

    restart {
      attempts = 0
      mode     = "fail"
    }

    reschedule {
      attempts  = 0
      unlimited = false
    }

    update {
      min_healthy_time = "5s"
    }
  }
}
