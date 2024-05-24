# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# This is a simple env job using the exec2 task driver.

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

    task "env" {
      driver = "exec2"

      config {
        command = "env"
      }

      resources {
        cpu    = 100
        memory = 64
      }
    }
  }
}
