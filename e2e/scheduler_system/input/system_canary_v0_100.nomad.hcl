# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "system_job" {
  datacenters = ["dc1", "dc2"]

  type = "system"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "system_job_group" {
    update {
      max_parallel     = 5
      min_healthy_time = "1s"
      healthy_deadline = "1m"
      auto_revert      = false
      canary           = 100
    }

    restart {
      attempts = 10
      interval = "1m"

      delay = "2s"
      mode  = "delay"
    }

    task "sleepy" {
      driver = "raw_exec"
      config {
        command = "/bin/sleep"
        args    = ["1000m"]
      }
    }
  }
}
