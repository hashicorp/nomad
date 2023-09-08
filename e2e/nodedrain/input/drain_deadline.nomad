# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "drain_deadline" {

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  migrate {
    max_parallel     = 1
    min_healthy_time = "30s"
  }

  group "group" {

    count = 2

    task "task" {
      driver = "docker"

      config {
        image   = "busybox:1"
        command = "/bin/sh"
        args    = ["-c", "sleep 600"]
      }

      resources {
        cpu    = 256
        memory = 64
      }
    }
  }
}
