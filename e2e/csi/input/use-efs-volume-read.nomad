# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# a job that mounts the EFS volume and sleeps, so that we can
# read its mounted file system remotely
job "use-efs-volume" {
  datacenters = ["dc1"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {

    volume "test" {
      type            = "csi"
      source          = "efs-vol0"
      attachment_mode = "file-system"
      access_mode     = "single-node-writer"
    }

    task "task" {
      driver = "docker"

      config {
        image   = "busybox:1"
        command = "/bin/sh"
        args    = ["-c", "sleep 3600"]
      }

      volume_mount {
        volume      = "test"
        destination = "${NOMAD_TASK_DIR}/test"
        read_only   = true
      }

      resources {
        cpu    = 500
        memory = 128
      }
    }
  }
}
