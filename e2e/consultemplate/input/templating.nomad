# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "templating" {
  datacenters = ["dc1", "dc2"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "docker_downstream" {

    task "task" {

      driver = "docker"

      config {
        image   = "busybox:1"
        command = "/bin/sh"
        args    = ["-c", "sleep 300"]
      }

      template {
        data = <<EOT
{{ range service "upstream-service" }}
server {{ .Name }} {{ .Address }}:{{ .Port }}{{ end }}
EOT

        destination = "${NOMAD_TASK_DIR}/services.conf"
        change_mode = "noop"
      }

      template {
        data = <<EOT
---
key: {{ key "consultemplatetest" }}
job: {{ env "NOMAD_JOB_NAME" }}
EOT

        destination = "${NOMAD_TASK_DIR}/kv.yml"
        change_mode = "restart"
      }

      resources {
        cpu    = 128
        memory = 64
      }
    }
  }

  group "exec_downstream" {

    task "task" {

      driver = "exec"

      config {
        command = "/bin/sh"
        args    = ["-c", "sleep 300"]
      }

      template {
        data = <<EOT
{{ range service "upstream-service" }}
server {{ .Name }} {{ .Address }}:{{ .Port }}{{ end }}
EOT

        destination = "${NOMAD_TASK_DIR}/services.conf"
        change_mode = "noop"
      }

      template {
        data        = <<EOT
---
key: {{ key "consultemplatetest" }}
job: {{ env "NOMAD_JOB_NAME" }}
EOT
        destination = "${NOMAD_TASK_DIR}/kv.yml"
        change_mode = "restart"
      }

      resources {
        cpu    = 128
        memory = 64
      }
    }
  }

  group "upstream" {

    count = 2

    network {
      mode = "bridge"
      port "web" {
        to = -1
      }
    }

    service {
      name = "upstream-service"
      port = "web"
    }

    task "task" {

      driver = "exec"

      config {
        command = "/bin/sh"
        args    = ["-c", "sleep 300"]
      }

      resources {
        cpu    = 128
        memory = 64
      }
    }
  }

}
