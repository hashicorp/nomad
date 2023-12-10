# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "test2" {

  datacenters = ["dc1", "dc2"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  type = "service"

  group "t2" {
    count = 3

    task "t2" {
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

    reschedule {
      attempts  = 2
      interval  = "5m"
      unlimited = false
    }
  }
}
