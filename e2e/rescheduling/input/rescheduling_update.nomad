# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "test4" {

  datacenters = ["dc1", "dc2"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  type = "service"

  group "t4" {
    count = 3

    task "t4" {
      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["-c", "sleep 5000"]
      }
    }

    update {
      max_parallel      = 1
      min_healthy_time  = "3s"
      auto_revert       = false
      healthy_deadline  = "5s"
      progress_deadline = "10s"
    }

    restart {
      attempts = 0
      delay    = "0s"
      mode     = "fail"
    }

    reschedule {
      attempts  = 3
      interval  = "5m"
      unlimited = false
    }
  }
}
