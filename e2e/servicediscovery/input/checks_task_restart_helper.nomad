# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "nodeID" {
  type = string
}

variable "cmd" {
  type = string
}

variable "delay" {
  type = string
}

job "checks_task_restart_helper" {
  datacenters = ["dc1"]
  type        = "batch"

  group "group" {

    constraint {
      attribute = "${attr.kernel.name}"
      value     = "linux"
    }

    constraint {
      attribute = "${node.unique_id}"
      value     = "${var.nodeID}"
    }

    reschedule {
      attempts  = 0
      unlimited = false
    }

    task "touch" {
      driver = "raw_exec"
      config {
        command = "bash"
        args    = ["-c", "sleep ${var.delay} && ${var.cmd} /tmp/nsd-checks-task-restart-test.txt"]
      }
      resources {
        cpu    = 50
        memory = 32
      }
    }
  }
}
