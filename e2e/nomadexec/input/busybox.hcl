# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "busybox" {
  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {
    task "task" {
      driver = "docker"

      config {
        image   = "busybox:1"
        command = "/bin/sleep"
        args    = ["infinity"]
      }

      resources {
        cpu    = 500
        memory = 128
      }
    }
  }
}

