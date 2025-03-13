# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "alloc_count" {
  type = number
}

job "get-secret" {
  group "group" {
    count = var.alloc_count

    network {
      port "web" {
        to = 8001
      }
    }

    service {
      provider = "consul"
      name     = "writes-vars-checker"
      port     = "web"
      task     = "task"

      /* check {
        type     = "script"
        interval = "10s"
        timeout  = "1s"
        command  = "/bin/sh"
        args     = ["/local/read-script.sh"]

        # this check will read from the Task API, so we need to ensure that we
        # can tolerate the listener going away during client upgrades
        check_restart {
          limit = 10
        }
      } */
    }

    task "read-secrets" {
      driver = "raw_exec"

      config {
        command = "/bin/bash"
        args    = ["-c", "cat local/config.json && sleep 30"]
      }

      vault {}

      template {
        destination = "local/config.json"
        change_mode = "restart"

        data = <<EOT
{{ with secret "{{}}/data/default/get-secret" }}
{
  "username": "{{ .Data.data.username }}",
  "password": "{{ .Data.data.password }}"
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
