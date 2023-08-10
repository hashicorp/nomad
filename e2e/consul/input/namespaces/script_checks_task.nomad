# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "script_checks_task" {
  datacenters = ["dc1"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group-a" {

    consul {
      namespace = "apple"
    }

    task "test" {
      service {
        name = "service-1a"

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
        name = "service-2a"

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
          name     = "alive-2ab"
          type     = "script"
          interval = "2s"
          timeout  = "2s"
          command  = "cat"
          args     = ["${NOMAD_TASK_DIR}/alive-2ab"]
        }
      }

      service {
        name = "service-3a"

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

  group "group-z" {

    # consul namespace not set

    task "test" {
      service {
        name = "service-1z"

        check {
          name     = "alive-1"
          type     = "script"
          interval = "2s"
          timeout  = "2s"
          command  = "echo"
          args     = ["alive-1z"]
        }
      }

      service {
        name = "service-2z"

        check {
          name     = "alive-2z"
          type     = "script"
          interval = "2s"
          timeout  = "2s"
          command  = "echo"
          args     = ["alive-2z"]
        }

        # the file expected by this check will not exist when started,
        # so the check will error-out and be in a warning state until
        # it's been created
        check {
          name     = "alive-2zb"
          type     = "script"
          interval = "2s"
          timeout  = "2s"
          command  = "cat"
          args     = ["${NOMAD_TASK_DIR}/alive-2zb"]
        }
      }

      service {
        name = "service-3z"

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
