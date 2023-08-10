# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "test1" {
  datacenters = ["dc1", "dc2"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  affinity {
    attribute = "${meta.rack}"
    operator  = "="
    value     = "r1"
    weight    = -50
  }

  group "test1" {
    count = 4

    affinity {
      attribute = "${node.datacenter}"
      operator  = "="
      value     = "dc1"
      weight    = -50
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
