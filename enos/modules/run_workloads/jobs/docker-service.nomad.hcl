# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "service-docker" {

  group "service-docker" {
    count = 3
    task "alpine" {
      driver = "docker"

      config {
        image   = "alpine:latest"
        command = "sh"
        args    = ["-c", "while true; do sleep 300; done"]

      }

      resources {
        cpu    = 100
        memory = 128
      }
    }
  }
}
