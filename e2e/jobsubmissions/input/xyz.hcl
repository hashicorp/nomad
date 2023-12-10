# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "X" {
  type = string
}

variable "Y" {
  type = number
}

variable "Z" {
  type = bool
}

job "xyz" {
  type = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {
    task "task" {
      driver = "raw_exec"

      config {
        command = "echo"
        args    = ["X ${var.X}, Y ${var.Y}, Z ${var.Z}"]
      }

      resources {
        cpu    = 10
        memory = 16
      }
    }
  }
}
