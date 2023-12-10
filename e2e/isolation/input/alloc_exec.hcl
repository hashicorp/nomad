# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "alloc_exec" {

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

    task "sleep" {
      driver = "exec"

      config {
        command = "sleep"
        args    = ["infinity"]
      }

      resources {
        cpu    = 10
        memory = 16
      }
    }
  }
}
