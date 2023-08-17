# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "networking" {
  datacenters = ["dc1", "dc2"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "bridged" {
    network {
      hostname = "mylittlepony"
      mode     = "bridge"
    }

    task "sleep" {
      driver = "docker"
      config {
        image   = "busybox:1"
        command = "/bin/sleep"
        args    = ["300"]
      }
    }
  }
}
