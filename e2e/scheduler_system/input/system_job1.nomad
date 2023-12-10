# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "system_job" {
  datacenters = ["dc1", "dc2"]

  type = "system"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "system_job_group" {
    count = 1

    restart {
      attempts = 10
      interval = "1m"

      delay = "2s"
      mode  = "delay"
    }

    task "system_task" {
      driver = "docker"

      config {
        image = "busybox:1"

        command = "/bin/sh"
        args    = ["-c", "sleep 15000"]
      }

      env {
        version = "2"
      }
    }
  }
}
