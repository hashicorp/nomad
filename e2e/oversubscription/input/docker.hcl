# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "oversubscription-docker" {
  type = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    operator  = "="
    value     = "linux"
  }

  constraint {
    attribute = "${attr.os.cgroups.version}"
    operator  = "="
    value     = "2"
  }

  group "group" {
    task "task" {
      driver = "docker"

      config {
        image   = "busybox:1"
        command = "/bin/sh"
        args    = ["-c", "cat /sys/fs/cgroup/memory.max; sleep infinity"]
      }

      resources {
        cpu        = 500
        memory     = 20
        memory_max = 30
      }
    }
  }
}
