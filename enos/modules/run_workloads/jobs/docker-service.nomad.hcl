# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1
variable "alloc_count" {
  type    = number
  default = 1
}

job "service-docker" {
  group "service-docker" {
    count = var.alloc_count

    network {
      port "db" {
        to = 6379
      }
    }

    service {
      provider = "consul"
      name     = "service-docker"
      port     = "db"

      check {
        name     = "service-docker_probe"
        type     = "tcp"
        interval = "10s"
        timeout  = "1s"
      }
    }

    task "service-docker" {
      driver = "docker"

      config {
        image = "redis:7.2"
        ports = ["db"]
        labels {
          workload = "docker-service"
        }
      }

      resources {
        cpu    = 50
        memory = 64
      }
    }
  }
}
