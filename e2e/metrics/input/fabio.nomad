# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "fabio" {
  datacenters = ["dc1", "dc2"]
  type        = "system"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "fabio" {
    network {
      port "lb" {
        static = 9999
      }

      port "ui" {
        static = 9998
      }
    }
    task "fabio" {
      driver = "docker"

      config {
        image        = "fabiolb/fabio"
        network_mode = "host"
      }

      resources {
        cpu    = 100
        memory = 64
      }
    }
  }
}
