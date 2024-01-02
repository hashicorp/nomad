# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "raw_exec" {
  datacenters = ["dc1"]
  type        = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "raw_exec" {
    task "raw_exec" {
      driver = "raw_exec"

      config {
        command = "bash"
        args = [
          "-c", "local/pid.sh"
        ]
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
        cpu    = 100
        memory = 64
      }
    }
  }
}
