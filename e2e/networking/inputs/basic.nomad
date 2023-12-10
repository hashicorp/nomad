# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

//e2e:service script=validate.sh
job "networking" {
  datacenters = ["dc1", "dc2"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "basic" {
    network {
      mode = "bridge"
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
