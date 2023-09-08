# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "nodeID" {
  type = string
}

variable "time" {
  type    = string
  default = "0"
}

job "restart-node" {
  type        = "batch"
  datacenters = ["dc1", "dc2"]

  group "group" {

    reschedule {
      attempts  = 0
      unlimited = false
    }

    # need to prevent the task from being restarted on reconnect, if
    # we're stopped long enough for the node to be marked down
    max_client_disconnect = "1h"

    constraint {
      attribute = "${attr.kernel.name}"
      value     = "linux"
    }
    constraint {
      attribute = "${node.unique.id}"
      value     = "${var.nodeID}"
    }

    task "task" {
      driver = "raw_exec"
      config {
        command = "/bin/sh"
        args = ["-c",
          # before disconnecting, we need to sleep long enough for the
          # task to register itself, otherwise we end up trying to
          # re-run the task immediately on reconnect
        "sleep 5; systemctl stop nomad; sleep ${var.time}; systemctl start nomad"]
      }
    }

  }
}
