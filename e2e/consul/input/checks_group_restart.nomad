# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "group_check_restart" {
  datacenters = ["dc1"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group_check_restart" {
    network {
      mode = "bridge"
    }

    restart {
      attempts = 2
      delay    = "1s"
      interval = "5m"
      mode     = "fail"
    }

    service {
      name = "group-service-1"
      port = "9003"

      # this check should always time out and so the service
      # should not be marked healthy, resulting in the tasks
      # getting restarted
      check {
        name     = "always-dead"
        type     = "script"
        task     = "fail"
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

    task "fail" {
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
