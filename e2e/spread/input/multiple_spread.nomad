# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "r1" {
  datacenters = ["dc1", "dc2"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  spread {
    attribute = "${node.datacenter}"
    weight    = 100
  }

  group "test1" {
    count = 10

    spread {
      attribute = "${meta.rack}"
      weight    = 100

      target "r1" {
        percent = 70
      }

      target "r2" {
        percent = 30
      }
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
