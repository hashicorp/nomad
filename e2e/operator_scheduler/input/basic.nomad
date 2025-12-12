# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

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
