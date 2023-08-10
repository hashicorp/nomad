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
    count = 1

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
      name = "script-check-svc"
      port = "db"

      check {
        name     = "tcp"
        type     = "tcp"
        port     = "db"
        interval = "10s"
        timeout  = "2s"
      }

      check {
        name      = "script-check-script"
        type      = "script"
        command   = "/bin/bash"
        interval  = "5s"
        timeout   = "1s"
        task      = "server"
        on_update = "ignore_warnings"

        args = [
          "-c",
          "/local/ready.sh"
        ]

        check_restart {
          limit           = 2
          ignore_warnings = true
        }
      }
    }


    task "server" {
      driver = "docker"

      config {
        image = "redis"
        ports = ["db"]
      }

      # Check script that reports as warning for long enough for deployment to
      # become healthy then errors
      template {
        data = <<EOT
#!/bin/sh

if [ ! -f /tmp/check_0 ]; then touch /tmp/check_0; exit 1; fi
if [ ! -f /tmp/check_1 ]; then touch /tmp/check_1; exit 1; fi
if [ ! -f /tmp/check_2 ]; then touch /tmp/check_2; exit 1; fi
if [ ! -f /tmp/check_3 ]; then touch /tmp/check_3; exit 1; fi
if [ ! -f /tmp/check_4 ]; then touch /tmp/check_4; exit 1; fi
if [ ! -f /tmp/check_5 ]; then touch /tmp/check_5; exit 1; fi
if [ ! -f /tmp/check_6 ]; then touch /tmp/check_6; exit 7; fi
if [ ! -f /tmp/check_7 ]; then touch /tmp/check_7; exit 7; fi
if [ ! -f /tmp/check_8 ]; then touch /tmp/check_8; exit 7; fi
if [ ! -f /tmp/check_9 ]; then touch /tmp/check_9; exit 7; fi


if [ -f /tmp/check_9 ]; then exit 7; fi
EOT

        destination = "local/ready.sh"
        perms       = "777"
      }
    }
  }
}

