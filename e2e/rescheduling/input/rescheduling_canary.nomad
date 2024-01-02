# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "test5" {

  datacenters = ["dc1", "dc2"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  type = "service"

  group "t5" {
    count = 3

    task "t5" {
      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["-c", "sleep 5000"]
      }
    }

    update {
      max_parallel     = 1
      canary           = 1
      min_healthy_time = "1s"
      auto_revert      = false
    }

    restart {
      attempts = 0
      delay    = "0s"
      mode     = "fail"
    }

    reschedule {
      attempts  = 3
      interval  = "5m"
      delay     = "5s"
      unlimited = false
    }
  }
}
