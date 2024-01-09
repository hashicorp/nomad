# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "oversubmax" {
  type = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {
    reschedule {
      attempts  = 0
      unlimited = false
    }

    restart {
      attempts = 0
      mode     = "fail"
    }

    task "cat" {
      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["-c", "cat /sys/fs/cgroup/$(cat /proc/self/cgroup | cut -d':' -f3)/memory.{low,max}"]
      }

      resources {
        cpu        = 100
        memory     = 64
        memory_max = -1 # unlimited
      }
    }
  }
}
