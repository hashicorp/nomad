# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "example" {
  datacenters = ["dc1"]

  group "cache" {
    volume "volume0" {
      type            = "csi"
      source          = "test-volume"
      attachment_mode = "file-system"
      access_mode     = "single-node-reader-only" # alt: "single-node-writer"
      read_only       = true
      per_alloc       = true
    }

    network {
      port "db" {
        to = 6379
      }
    }

    task "redis" {
      driver = "podman"

      config {
        image = "docker://redis:7"
        ports = ["db"]
      }

      volume_mount {
        volume      = "volume0"
        destination = "${NOMAD_ALLOC_DIR}/volume0"
      }

      resources {
        cpu    = 100
        memory = 100
      }
    }
  }
}
