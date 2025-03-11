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

    network {
      port "db" {
        to = 6377
      }
    }

    service {
      provider = "consul"
      name     = "batch-docker"
      port     = "db"

      check {
        name     = "service-docker_probe"
        type     = "tcp"
        interval = "10s"
        timeout  = "1s"
      }
    }

    task "batch-docker" {
      driver = "docker"

      config {
        image = "redis:latest"
        ports = ["db"]
        args  = ["--port", "6377"]
        labels {
          workload = "docker-batch"
        }
      }

      resources {
        cpu    = 50
        memory = 64
      }
    }
  }
}
