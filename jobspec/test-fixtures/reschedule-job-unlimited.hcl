# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


job "foo" {
  datacenters = ["dc1"]
  type        = "batch"

  reschedule {
    delay          = "10s"
    delay_function = "exponential"
    max_delay      = "120s"
    unlimited      = true
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
