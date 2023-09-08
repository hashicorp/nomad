# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "failed_batch" {
  type        = "batch"
  datacenters = ["dc1"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "failed_batch" {
    restart {
      attempts = 0
    }

    task "failed_batch" {
      driver = "raw_exec"

      config {
        command = "SomeInvalidCommand"
      }
    }
  }
}
