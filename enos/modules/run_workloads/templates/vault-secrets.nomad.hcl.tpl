# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "alloc_count" {
  type = number
}

job "get-secret" {
  group "group" {
    count = var.alloc_count

  restart {
    interval         = "5s"
    delay            = "1s"
    mode             = "delay"
    render_templates = true
  }

    network {
      port "web" {
        to = 8001
      }
    }

    service {
      provider = "consul"
      name     = "get-secret"
      port     = "web"
      task     = "read-secrets"

    check {
        interval = "10s"
        timeout  = "1s"

    type    = "script"
    command = "/bin/bash"
    args    = ["-c", "test -f local/config.json"]

      }
    }

    task "read-secrets" {
      driver = "raw_exec"

     config {
        command = "/bin/bash"
        args    = ["-c", "while true; do cat local/config.json; sleep 1; done"]
      }

      vault {}

      template {
        destination = "local/config.json"
        change_mode   = "signal"
        change_signal = "SIGHUP"

        data = <<EOT
{{ with secret "${secret_path}" }}
{
  "username": "{{ .Data.data.username }}",
  "password": "{{ .Data.data.password }}"
  {{ timestamp "unix" }}
}
{{ end }}
EOT
      }

      identity {
        env = true
      }

      resources {
        cpu    = 100
        memory = 64
      }
    }
  }
}
