# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# this variable is not used but required by runner; we have single-node-writer
# set so we only ever want a single allocation for this job
variable "alloc_count" {
  type    = number
  default = 1
}

# a job that mounts an EFS volume and writes its job ID as a file
job "wants-efs-volume" {

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {

    volume "test" {
      type            = "csi"
      source          = "efsTestVolume"
      attachment_mode = "file-system"
      access_mode     = "single-node-writer"
    }

    task "task" {
      driver = "docker"

      config {
        image   = "busybox:1"
        command = "httpd"
        args    = ["-vv", "-f", "-p", "8001", "-h", "/local"]
      }

      volume_mount {
        volume      = "test"
        destination = "${NOMAD_TASK_DIR}/test"
        read_only   = false
      }

      resources {
        cpu    = 100
        memory = 64
      }
    }

    task "sidecar" {
      driver = "docker"

      config {
        image   = "busybox:1"
        command = "/bin/sh"
        args    = ["-c", "echo '${NOMAD_ALLOC_ID}' > ${NOMAD_TASK_DIR}/index.html"]
      }

      lifecycle {
        hook    = "poststart"
        sidecar = false
      }

      volume_mount {
        volume      = "test"
        destination = "${NOMAD_TASK_DIR}/test"
        read_only   = false
      }

      resources {
        cpu    = 10
        memory = 10
      }

    }
  }
}
