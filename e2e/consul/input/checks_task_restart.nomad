# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "task_check" {
  datacenters = ["dc1"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "task_check" {
    count = 1

    restart {
      attempts = 2
      delay    = "1s"
      interval = "5m"
      mode     = "fail"
    }

    task "fail" {

      service {
        name = "task-service-1"

        # this check should always time out and so the service
        # should not be marked healthy
        check {
          name     = "always-dead"
          type     = "script"
          interval = "2s"
          timeout  = "1s"
          command  = "sleep"
          args     = ["10"]

          check_restart {
            limit           = 2
            grace           = "5s"
            ignore_warnings = false
          }

        }
      }

      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["-c", "sleep 15000"]
      }
    }


    task "ok" {
      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["-c", "sleep 15000"]
      }
    }

  }
}
