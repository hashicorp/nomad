# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "identitycompat" {
  group "cache" {
    count = 1

    network {
      port "db" {
        to = 6379
      }
    }

    task "redis" {
      driver = "docker"

      config {
        image          = "redis:7"
        ports          = ["db"]
        auth_soft_fail = true
      }

      identity {
        env  = true
        file = true
      }

      # This identity will only be supported by >=1.7 agents but is included to
      # ensure parsing handles both the default and alternate identities
      # properly.
      identity {
        name = "foo"
        aud  = ["bar"]
      }

      resources {
        cpu    = 400
        memory = 256 # 256MB
      }
    }
  }
}
