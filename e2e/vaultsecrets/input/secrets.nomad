# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "secrets" {
  datacenters = ["dc1", "dc2"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {

    meta {
      test_deploy = "DEPLOYNUMBER"
    }

    task "task" {

      driver = "docker"

      config {
        image   = "busybox:1"
        command = "/bin/sh"
        args    = ["-c", "sleep 300"]
      }

      vault {
        policies = ["access-secrets-TESTID"]
      }

      template {
        data = <<EOT
{{ with secret "pki-TESTID/issue/nomad" "common_name=nomad.service.consul" "ip_sans=127.0.0.1" }}
{{- .Data.certificate -}}
{{ end }}
EOT

        destination = "${NOMAD_SECRETS_DIR}/certificate.crt"
        change_mode = "noop"
      }

      template {
        data = <<EOT
SOME_SECRET={{ with secret "secrets-TESTID/data/myapp" }}{{- .Data.data.key -}}{{end}}
EOT

        destination = "${NOMAD_SECRETS_DIR}/access.key"
      }

      resources {
        cpu    = 128
        memory = 64
      }
    }

  }
}
