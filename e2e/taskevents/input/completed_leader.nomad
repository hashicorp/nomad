# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "completed_leader" {
  type        = "batch"
  datacenters = ["dc1"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "completed_leader" {
    restart {
      attempts = 0
    }

    # Only the task named the same as the job has its events tested.
    task "completed_leader" {
      driver = "raw_exec"

      config {
        command = "sleep"
        args    = ["1000"]
      }
    }

    task "leader" {
      leader = true
      driver = "raw_exec"

      config {
        command = "sleep"
        args    = ["1"]
      }
    }
  }
}
