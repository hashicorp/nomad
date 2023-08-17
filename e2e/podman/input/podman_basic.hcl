# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "podman_basic" {

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "cache" {
    network {
      port "db" {
        to = 6379
      }
    }

    task "redis" {
      driver = "podman"

      config {
        image = "redis:7"
        ports = ["db"]
      }

      resources {
        cpu    = 50
        memory = 128
      }
    }
  }
}
