# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "default_wi" {

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {

    task "task" {

      driver = "docker"

      config {
        image   = "busybox:1"
        command = "/bin/sh"
        args    = ["-c", "sleep 300"]
      }

      vault {}

      template {
        data = <<EOT
E2E_SECRET={{ with secret "SECRET_PATH" }}{{- .Data.data.key -}}{{end}}
EOT

        destination = "${NOMAD_SECRETS_DIR}/secret.txt"
        env         = true
      }

      resources {
        cpu    = 128
        memory = 64
      }
    }
  }
}
