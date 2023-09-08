# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "diskstress" {
  datacenters = ["dc1", "dc2"]
  type        = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "diskstress" {
    count = 1

    task "diskstress" {
      driver = "docker"

      config {
        image = "progrium/stress"

        args = [
          "--hdd",
          "2",
          "--timeout",
          "30",
        ]
      }

      resources {
        cpu    = 1024
        memory = 256
      }
    }
  }
}
