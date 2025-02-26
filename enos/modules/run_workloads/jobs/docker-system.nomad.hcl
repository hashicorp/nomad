# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1
variable "alloc_count" {
  type    = number
  default = 1
}

job "system-docker" {
  type = "system"

  group "system-docker" {

    network {
      port "db" {
        to = 6378
      }
    }

    service {
      provider = "consul"
      name     = "system-docker"
      port     = "db"

      check {
        name     = "system-docker_probe"
        type     = "tcp"
        interval = "10s"
        timeout  = "1s"
      }
    }


    task "system-docker" {
      driver = "docker"

      config {
        image = "redis:7.2"
        ports = ["db"]
        args  = ["--port", "6378"]
        labels {
          workload = "docker-system"
        }
      }

      resources {
        cpu    = 50
        memory = 64
      }
    }
  }
}
