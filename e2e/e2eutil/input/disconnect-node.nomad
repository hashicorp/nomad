variable "nodeID" {
  type = string
}

variable "time" {
  type    = string
  default = "0"
}

job "disconnect-node" {
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
        "iptables -I OUTPUT -p tcp --dport 4647 -j DROP; sleep ${var.time}; iptables -D OUTPUT -p tcp --dport 4647 -j DROP"]
      }
    }

  }
}
