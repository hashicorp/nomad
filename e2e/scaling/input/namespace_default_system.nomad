# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "system_job" {
  type = "system"

  group "system_job_group" {

    task "system_task" {
      driver = "docker"

      config {
        image = "busybox:1"

        command = "/bin/sh"
        args    = ["-c", "sleep 15000"]
      }

      env {
        version = "1"
      }
    }
  }
}

