# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "exec_host" {
  type = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {
    reschedule {
      attempts  = 0
      unlimited = false
    }

    restart {
      attempts = 0
      mode     = "fail"
    }

    task "bash" {
      driver = "exec"

      config {
        command = "bash"
        args = [
          "-c", "local/pid.sh"
        ]
        pid_mode = "host"
        ipc_mode = "host"
      }

      template {
        data = <<EOF
#!/usr/bin/env bash
echo my pid is $BASHPID
EOF

        destination = "local/pid.sh"
        perms       = "777"
        change_mode = "noop"
      }

      resources {
        cpu    = 10
        memory = 16
      }
    }
  }
}
