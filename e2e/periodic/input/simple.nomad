# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "periodic" {
  datacenters = ["dc1"]
  type        = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    operator  = "set_contains_any"
    value     = "darwin,linux"
  }



  periodic {
    cron             = "* * * * *"
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
