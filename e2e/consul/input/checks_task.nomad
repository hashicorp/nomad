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

        check {
          name     = "alive-1"
          type     = "script"
          interval = "2s"
          timeout  = "2s"
          command  = "echo"
          args     = ["alive-1"]
        }
      }

      service {
        name = "task-service-2"

        check {
          name     = "alive-2a"
          type     = "script"
          interval = "2s"
          timeout  = "2s"
          command  = "echo"
          args     = ["alive-2a"]
        }

        # the file expected by this check will not exist when started,
        # so the check will error-out and be in a warning state until
        # it's been created
        check {
          name     = "alive-2b"
          type     = "script"
          interval = "2s"
          timeout  = "2s"
          command  = "cat"
          args     = ["${NOMAD_TASK_DIR}/alive-2b"]
        }
      }

      service {
        name = "task-service-3"

        # this check should always time out and so the service
        # should not be marked healthy
        check {
          name     = "always-dead"
          type     = "script"
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
