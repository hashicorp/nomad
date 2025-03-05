# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "alloc_count" {
  type    = number
  default = 1
}

job "nfs" {
  group "nfs" {
    count = var.alloc_count

    volume "host-nfs" {
      type   = "host"
      source = "shared_data"
    }

    service {
      name     = "nfs"
      port     = "nfs"
      provider = "nomad"

      check {
        type     = "tcp"
        interval = "10s"
        timeout  = "1s"
      }
    }

    network {
      mode = "host"
      port "nfs" {
        static = 2049
        to     = 2049
      }
    }

    task "nfs" {
      driver = "docker"
      config {
        image      = "atlassian/nfs-server-test:2.1"
        ports      = ["nfs"]
        privileged = true
      }

      env {
        EXPORT_PATH = "/srv/nfs"
      }

      volume_mount {
        volume      = "host-nfs"
        destination = "/srv/nfs"
      }
    }
  }
}
