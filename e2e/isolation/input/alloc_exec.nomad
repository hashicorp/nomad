# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "alloc_exec" {
  datacenters = ["dc1"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "main" {
    task "main" {
      driver = "exec"

      config {
        command = "/bin/sleep"
        args    = ["30s"]
      }

      resources {
        cpu    = 100
        memory = 64
      }
    }
  }
}
