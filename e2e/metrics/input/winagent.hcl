# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "winagent" {
  type = "system"

  group "windows" {

    constraint {
      attribute = "${attr.kernel.name}"
      value     = "windows"
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
      port "api" {
        static = 6120
      }
    }

    service {
      provider = "nomad"
      name     = "holepunch"
      port     = "api"
      tags     = ["monitor"]
      check {
        type     = "http"
        path     = "/health"
        interval = "10s"
        timeout  = "2s"
      }
    }

    task "task" {
      driver = "raw_exec"

      artifact {
        source = "https://github.com/shoenig/nomad-holepunch/releases/download/v0.1.3/nomad-holepunch_0.1.3_windows_amd64.tar.gz"
        destination = "local/"
      }

      config {
        command = "./nomad-holepunch.exe"
      }

      env {
        HOLEPUNCH_BIND = "0.0.0.0"
        HOLEPUNCH_PORT = "${NOMAD_PORT_api}"
      }

      identity {
        env = true
      }

      resources {
        cpu    = 100
        memory = 128
      }
    }
  }
}

