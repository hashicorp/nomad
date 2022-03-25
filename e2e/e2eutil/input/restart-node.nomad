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
