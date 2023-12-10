# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "failed_sibling" {
  type        = "service"
  datacenters = ["dc1"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "failed_sibling" {
    restart {
      attempts = 0
    }

    # Only the task named the same as the job has its events tested.
    task "failed_sibling" {
      driver = "raw_exec"

      config {
        command = "sleep"
        args    = ["1000"]
      }
    }

    task "failure" {
      driver = "raw_exec"

      config {
        command = "/bin/sh"
        args    = ["-c", "sleep 1 && exit 99"]
      }
    }
  }
}
