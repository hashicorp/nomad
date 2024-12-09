# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "job" {
  group "g" {
    task "t" {
      driver = "docker"
      config {
        image   = "python:slim"
        command = "bash"
        args = ["-xc", <<-EOF
          for dir in internal external; do
            touch ${NOMAD_TASK_DIR}/$dir/hiii
          done
          python -m http.server -d ${NOMAD_TASK_DIR} --bind=::
          EOF
        ]
        ports = ["http"]
      }
      volume_mount {
        volume      = "int"
        destination = "${NOMAD_TASK_DIR}/internal"
      }
      volume_mount {
        volume      = "ext"
        destination = "${NOMAD_TASK_DIR}/external"
      }
    }
    volume "int" {
      type   = "host"
      source = "internal-plugin"
    }
    volume "ext" {
      type   = "host"
      source = "external-plugin"
    }
    network {
      port "http" {
        static = 8000
      }
    }
    service {
      name     = "job"
      port     = "http"
      provider = "nomad"
    }
  }
}
