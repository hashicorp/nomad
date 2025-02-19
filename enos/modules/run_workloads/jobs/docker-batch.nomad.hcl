# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1
variable "alloc_count" {
  type    = number
  default = 1
}

job "batch-docker" {
  type = "batch"

  group "batch-docker" {
    count = var.alloc_count

    task "batch-docker" {
      driver = "docker"

      config {
        image   = "alpine:latest"
        command = "sh"
        args    = ["-c", "while true; do sleep 30000; done"]

      }

      resources {
        cpu    = 50
        memory = 64
      }
    }
  }
}
