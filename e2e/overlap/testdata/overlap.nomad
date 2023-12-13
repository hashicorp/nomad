# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "overlap" {
  datacenters = ["dc1", "dc2"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "overlap" {
    count = 1

    network {
      # Reserve a static port so that the subsequent job run is blocked until
      # the port is freed
      port "hack" {
        static = 7234
      }
    }

    task "test" {
      driver = "raw_exec"

      # Delay shutdown to delay next placement
      shutdown_delay = "8s"

      config {
        command = "bash"
        args    = ["-c", "sleep 15000"]
      }

      resources {
        cpu    = "500"
        memory = "100"
      }
    }
  }
}

