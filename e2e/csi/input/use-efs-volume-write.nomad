# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# a job that mounts an EFS volume and writes its job ID as a file
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
        args    = ["-c", "echo 'ok' > ${NOMAD_TASK_DIR}/test/${NOMAD_ALLOC_ID}; sleep 3600"]
      }

      volume_mount {
        volume      = "test"
        destination = "${NOMAD_TASK_DIR}/test"
        read_only   = false
      }

      resources {
        cpu    = 500
        memory = 128
      }
    }
  }
}
