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
        "systemctl stop nomad; sleep ${var.time}; systemctl start nomad"]
      }
    }

  }
}
