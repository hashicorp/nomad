# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "drain_migrate" {

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {

    ephemeral_disk {
      migrate = true
      size    = "101"
    }

    task "task" {
      driver = "docker"

      config {
        image   = "busybox:1"
        command = "/bin/sh"
        args    = ["-c", "echo \"data from $NOMAD_ALLOC_ID\" >> /alloc/data/migrate.txt && sleep 120"]
      }

      resources {
        cpu    = 256
        memory = 128
      }
    }
  }
}
