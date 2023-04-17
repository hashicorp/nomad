# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "operator_scheduler" {
  datacenters = ["dc1"]
  type        = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "operator_scheduler" {

    task "test" {
      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["-c", "sleep 30"]
      }
    }
  }
}
