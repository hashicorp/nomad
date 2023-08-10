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

    task "test" {
      service {
        name = "task-service-1"

        # after update, check name has changed
        check {
          name     = "alive-1a"
          type     = "script"
          task     = "test"
          interval = "2s"
          timeout  = "2s"
          command  = "echo"
          args     = ["alive-1a"]
        }
      }

      service {
        name = "task-service-2"

        check {
          name     = "alive-2a"
          type     = "script"
          task     = "test"
          interval = "2s"
          timeout  = "2s"
          command  = "echo"
          args     = ["alive-2a"]
        }

        # after updating, this check will always pass
        check {
          name     = "alive-2b"
          type     = "script"
          task     = "test"
          interval = "2s"
          timeout  = "2s"
          command  = "echo"
          args     = ["alive-2b"]
        }
      }

      service {
        name = "task-service-3"

        # this check should always time out and so the service
        # should not be marked healthy
        check {
          name     = "always-dead"
          type     = "script"
          task     = "test"
          interval = "2s"
          timeout  = "1s"
          command  = "sleep"
          args     = ["10"]
        }
      }

      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["-c", "sleep 15000"]
      }
    }
  }
}
