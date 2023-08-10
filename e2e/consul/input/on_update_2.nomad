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
      health_check      = "checks"
      progress_deadline = "45s"
      healthy_deadline  = "30s"
    }

    service {
      name = "echo-service"
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
        on_update = "ignore"

        args = [
          "-c",
          "echo 'this check errors'; exit 2;",
        ]

      }
    }

    task "server" {
      driver = "docker"

      env {
        a = "b"
      }

      config {
        image = "redis"
        ports = ["db"]
      }
    }
  }
}


