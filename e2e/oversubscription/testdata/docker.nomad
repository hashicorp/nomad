# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "oversubscription-docker" {
  datacenters = ["dc1"]

  constraint {
    attribute = "${attr.kernel.name}"
    operator  = "set_contains_any"
    value     = "darwin,linux"
  }

  constraint {
    attribute = "${attr.unique.cgroup.version}"
    operator  = "="
    value     = "v2"
  }

  group "group" {
    task "task" {
      driver = "docker"

      config {
        image   = "busybox:1.29.2"
        command = "/bin/sh"
        args    = ["-c", "cat /sys/fs/cgroup/memory.max; sleep 1000"]
      }

      resources {
        cpu        = 500
        memory     = 20
        memory_max = 30
      }
    }
  }
}
