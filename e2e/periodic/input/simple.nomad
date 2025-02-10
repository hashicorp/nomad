# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "periodic" {
  type = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    operator  = "set_contains_any"
    value     = "darwin,linux"
  }

  periodic {
    # run on Jan 31st at 13:13, only if it's Sunday, to ensure no collisions
    # with our test forcing a dispatch
    cron             = "13 13 31 1 7"
    prohibit_overlap = true
  }

  group "group" {
    task "task" {
      driver = "docker"

      config {
        image   = "busybox:1"
        command = "/bin/sh"
        args    = ["-c", "sleep 5"]
      }
    }
  }
}
