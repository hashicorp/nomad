# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "test" {
  datacenters = ["dc1"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "test" {
    count = 3

    network {
      port "db" {
        to = 6379
      }
    }

    update {
      health_check = "checks"
    }

    service {
      name = "on-update-service"
      port = "db"

      check {
        name     = "tcp"
        type     = "tcp"
        port     = "db"
        interval = "10s"
        timeout  = "2s"
      }

      check {
        name      = "script-check"
        type      = "script"
        command   = "/bin/bash"
        interval  = "30s"
        timeout   = "10s"
        task      = "server"
        on_update = "ignore_warnings"

        args = [
          "-c",
          "echo 'this check warns'; exit 1;",
        ]

      }
    }

    task "server" {
      driver = "docker"

      env {
        a = "a"
      }

      config {
        image = "redis"
        ports = ["db"]
      }
    }
  }
}

