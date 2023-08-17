# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "r1" {
  datacenters = ["dc1", "dc2"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "r1" {
    count = 6

    spread {
      attribute = "${node.datacenter}"
      weight    = 100
    }

    task "test" {
      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["-c", "sleep 15000"]
      }
    }
  }
}
