# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "test" {

  datacenters = ["dc1", "dc2"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  type = "system"

  group "t" {
    count = 1

    task "t" {
      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["-c", "lol 5000"]
      }
    }

    restart {
      attempts = 0
      delay    = "0s"
      mode     = "fail"
    }
  }
}
