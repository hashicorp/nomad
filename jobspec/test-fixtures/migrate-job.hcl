# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "foo" {
  datacenters = ["dc1"]
  type        = "batch"

  migrate {
    max_parallel     = 2
    health_check     = "task_states"
    min_healthy_time = "11s"
    healthy_deadline = "11m"
  }

  group "bar" {
    count = 3

    task "bar" {
      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["-c", "echo hi"]
      }
    }

    migrate {
      max_parallel     = 3
      health_check     = "checks"
      min_healthy_time = "1s"
      healthy_deadline = "1m"
    }
  }
}
