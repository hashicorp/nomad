# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "events" {
  datacenters = ["dc1"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "one" {
    count = 3

    update {
      max_parallel     = 3
      auto_promote     = true
      canary           = 2
      min_healthy_time = "1s"
    }

    task "one" {
      driver = "raw_exec"

      env {
        version = "1"
      }

      config {
        command = "/bin/sleep"

        # change args to update the job, the only changes
        args = ["1000000"]
      }

      resources {
        cpu    = 20
        memory = 2000000
      }
    }
  }
}

