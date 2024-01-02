# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "deployment_auto.nomad" {
  datacenters = ["dc1"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "one" {
    count = 3

    update {
      max_parallel = 3
      auto_promote = true
      canary       = 2
    }

    task "one" {
      driver = "raw_exec"

      config {
        command = "/bin/sleep"

        # change args to update the job, the only changes
        args = ["1000001"]
      }

      resources {
        cpu    = 20
        memory = 20
      }
    }
  }

  group "two" {
    count = 3

    update {
      max_parallel     = 2
      auto_promote     = true
      canary           = 2
      min_healthy_time = "2s"
    }

    task "two" {
      driver = "raw_exec"

      config {
        command = "/bin/sleep"

        # change args to update the job, the only changes
        args = ["2000001"]
      }

      resources {
        cpu    = 20
        memory = 20
      }
    }
  }
}
