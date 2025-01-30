# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1
variable "alloc_count" {
  type    = number
  default = 1
}

job "service-docker" {

  group "service-docker" {
    count = var.alloc_count
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
