# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "format" {}

job "cni_config_format" {
  group "group" {
    network {
      mode = "cni/test-loopback-${var.format}"
    }
    task "task" {
      driver = "raw_exec"
      config {
        command = "sleep"
        args    = ["300"]
      }
    }

    # go faster
    update {
      min_healthy_time = "0s"
    }
    # fail faster (if it does fail)
    reschedule {
      attempts  = 0
      unlimited = false
    }
    restart {
      attempts = 0
      mode     = "fail"
    }
  }
}
