# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# This "sleep" job simply invokes 'sleep infinity' using raw_exec. It is great
# for demonstrating features of the Nomad e2e suite with a trivial job spec.

job "sleep" {

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {

    update {
      min_healthy_time = "2s"
    }

    reschedule {
      attempts  = 0
      unlimited = false
    }

    restart {
      attempts = 0
      mode     = "fail"
    }

    task "task" {
      driver = "raw_exec"

      config {
        command = "sleep"
        args    = ["infinity"]
      }

      resources {
        cpu    = 10
        memory = 10
      }
    }
  }
}
