# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# This "sleepytar" job just sleeps, but is used as a target for a nomad alloc
# exec API invocation to run a tar job that reads its data from stdin.

job "sleepytar" {
  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {
    update {
      min_healthy_time = "3s"
    }

    reschedule {
      unlimited = false
      attempts  = 0
    }

    restart {
      attempts = 0
      mode     = "fail"
    }

    task "task" {
      driver = "docker"

      config {
        image        = "bash:latest"
        command      = "sleep"
        args         = ["infinity"]
        network_mode = "none"
      }

      resources {
        cores  = 1
        memory = 128
      }
    }
  }
}

