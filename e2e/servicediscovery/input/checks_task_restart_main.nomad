# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

variable "filename" {
  type = string
}

job "checks_task_restart" {
  type = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {
    network {
      mode = "host"
      port "http" {}
    }

    service {
      provider = "nomad"
      name     = "nsd-checks-task-restart-test"
      port     = "http"
      check {
        name     = "alive"
        type     = "http"
        path     = "/${var.filename}"
        interval = "2s"
        timeout  = "1s"
        check_restart {
          limit = 10
          grace = "1s"
        }
      }
    }

    task "python" {
      driver = "raw_exec"
      config {
        command = "python3"
        args    = ["-m", "http.server", "${NOMAD_PORT_http}", "--directory", "/tmp"]
      }
      resources {
        cpu    = 50
        memory = 64
      }
    }
  }
}
