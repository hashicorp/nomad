# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "checks_task_restart" {
  datacenters = ["dc1"]
  type        = "service"

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
        path     = "/nsd-checks-task-restart-test.txt"
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
