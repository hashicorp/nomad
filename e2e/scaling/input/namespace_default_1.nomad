# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "horizontally_scalable" {
  datacenters = ["dc1"]
  type        = "service"

  update {
    health_check = "task_states"
  }

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "horizontally_scalable" {

    scaling {
      min     = 2
      max     = 3
      enabled = true

      policy {}
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
