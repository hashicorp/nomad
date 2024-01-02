# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


job "foo" {
  datacenters = ["dc1"]
  type        = "batch"

  reschedule {
    attempts       = 15
    interval       = "30m"
    delay          = "10s"
    delay_function = "constant"
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
  }
}
