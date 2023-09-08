# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "nomadexec-docker" {
  datacenters = ["dc1"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {
    task "task" {
      driver = "docker"

      config {
        image   = "busybox:1.29.2"
        command = "/bin/sleep"
        args    = ["1000"]
      }

      resources {
        cpu    = 500
        memory = 256
      }
    }
  }
}
