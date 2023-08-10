# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "foo" {
  datacenters = ["dc1"]

  group "bar" {
    count          = 3
    shutdown_delay = "14s"

    network {
      mode     = "bridge"
      hostname = "foobar"

      port "http" {
        static       = 80
        to           = 8080
        host_network = "public"
      }
    }

    task "bar" {
      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["-c", "echo hi"]
      }

      resources {
        network {
          mbits = 10
        }
      }
    }
  }
}
