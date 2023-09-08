# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "alloc_zombie" {

  group "group" {
    network {
      mode = "host"
      port "http" {}
    }

    service {
      name     = "alloczombie"
      port     = "http"
      provider = "nomad"

      check {
        name     = "alloczombiecheck"
        type     = "http"
        port     = "http"
        path     = "/does/not/exist.txt"
        interval = "2s"
        timeout  = "1s"
        check_restart {
          limit = 1
          grace = "3s"
        }
      }
    }

    reschedule {
      attempts       = 3
      interval       = "1m"
      delay          = "5s"
      delay_function = "constant"
      unlimited      = false
    }

    restart {
      attempts = 0
      delay    = "5s"
      mode     = "fail"
    }

    task "python" {
      driver = "raw_exec"

      config {
        command = "python3"
        args    = ["-m", "http.server", "${NOMAD_PORT_http}", "--directory", "/tmp"]
      }

      resources {
        cpu    = 10
        memory = 64
      }
    }
  }
}
