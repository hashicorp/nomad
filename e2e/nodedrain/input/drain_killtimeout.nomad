# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "drain_killtimeout" {

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {

    task "task" {
      driver = "docker"

      kill_timeout = "30s" # matches the agent's max_kill_timeout

      config {
        image   = "busybox:1"
        command = "/bin/sh"
        args    = ["local/script.sh"]
      }

      # this job traps SIGINT so that we can assert that we've forced the drain
      # to wait until the client status has been updated
      template {
        data = <<EOF
#!/bin/sh
trap 'sleep 60' 2
sleep 600
EOF

        destination = "local/script.sh"
        change_mode = "noop"
      }

      resources {
        cpu    = 256
        memory = 128
      }
    }
  }
}
