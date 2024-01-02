# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "image" { default = "busybox:1" }

job "networking" {
  datacenters = ["dc1", "dc2"]
  priority    = 13
  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }
  group "bridged" {
    task "sleep" {
      driver = "docker"
      config {
        image   = var.image
        command = "/bin/sleep"
        args    = ["300"]
      }
    }
  }
}
