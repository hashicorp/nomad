# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "test1" {

  datacenters = ["dc1", "dc2"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  type = "service"

  group "t1" {
    count = 3

    task "t1" {
      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["-c", "lol 5000"]
      }
    }

    update {
      max_parallel     = 1
      min_healthy_time = "10s"
      auto_revert      = false
    }

    restart {
      attempts = 0
      delay    = "0s"
      mode     = "fail"
    }

    reschedule {
      attempts  = 0
      interval  = "5m"
      unlimited = false
    }
  }
}
