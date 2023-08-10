# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "binstore-storagelocker" {
  group "binsl" {
    ephemeral_disk {
      size = 500
    }

    ephemeral_disk {
      size = 100
    }

    count = 5

    task "binstore" {
      driver = "docker"

      resources {
        cpu    = 500
        memory = 128
      }

      resources {
        cpu    = 500
        memory = 128
      }
    }
  }
}
