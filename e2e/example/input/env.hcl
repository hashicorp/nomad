# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

# This "env" job simply invokes 'env' using raw_exec.

job "env" {
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

    task "task" {
      user   = "nobody"
      driver = "raw_exec"

      config {
        command = "env"
      }

      resources {
        cpu    = 10
        memory = 10
      }
    }
  }
}
