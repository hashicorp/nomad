# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# Sleeper is a fake service that outputs its pid to a file named `pid` to
# assert duplicate tasks are never started.

job "sleeper" {
  datacenters = ["dc1"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  task "sleeper" {
    driver = "raw_exec"

    config {
      command = "/bin/bash"
      args    = ["-c", "echo $$ >> pid && sleep 999999"]
    }
  }
}
