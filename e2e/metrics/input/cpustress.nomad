# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "cpustress" {
  datacenters = ["dc1", "dc2"]
  type        = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "cpustress" {
    count = 1

    task "cpustress" {
      driver = "docker"

      config {
        image = "progrium/stress"

        args = [
          "--cpu",
          "2",
          "--timeout",
          "600",
        ]
      }

      resources {
        cpu    = 2056
        memory = 256
      }
    }
  }
}
