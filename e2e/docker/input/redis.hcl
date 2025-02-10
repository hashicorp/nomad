# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# This is a simple redis job using the docker task driver.

job "redis" {

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
      driver = "docker"

      config {
        image          = "docker.io/library/redis:7"
        ports          = ["db"]
        auth_soft_fail = true
      }

      resources {
        cpu    = 50
        memory = 128
      }
    }
  }
}
