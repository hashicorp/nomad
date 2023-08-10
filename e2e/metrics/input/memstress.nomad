# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "memstress" {
  datacenters = ["dc1", "dc2"]
  type        = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "memstress" {
    count = 1

    task "memstress" {
      driver = "docker"

      config {
        image = "progrium/stress"

        args = [
          "--vm",
          "2",
          "--vm-bytes",
          "128M",
          "--timeout",
          "120",
        ]
      }

      resources {
        cpu    = 1024
        memory = 256
      }
    }
  }
}
