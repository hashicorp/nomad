# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "script_checks_group" {
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

    network {
      mode = "bridge"
    }

    service {
      name = "service-1a"
      port = "9001"

      check {
        name     = "alive-1"
        type     = "script"
        task     = "test"
        interval = "2s"
        timeout  = "2s"
        command  = "echo"
        args     = ["alive-1"]
      }
    }

    service {
      name = "service-2a"
      port = "9002"

      check {
        name     = "alive-2a"
        type     = "script"
        task     = "test"
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
        task     = "test"
        interval = "2s"
        timeout  = "2s"
        command  = "cat"
        args     = ["/tmp/${NOMAD_ALLOC_ID}-alive-2ab"]
      }
    }

    service {
      name = "service-3a"
      port = "9003"

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

    task "test" {
      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["-c", "sleep 15000"]
      }
    }
  }

  group "group-z" {

    # no consul namespace set

    network {
      mode = "bridge"
    }

    service {
      name = "service-1z"
      port = "9001"

      check {
        name     = "alive-1z"
        type     = "script"
        task     = "test"
        interval = "2s"
        timeout  = "2s"
        command  = "echo"
        args     = ["alive-1"]
      }
    }

    service {
      name = "service-2z"
      port = "9002"

      check {
        name     = "alive-2z"
        type     = "script"
        task     = "test"
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
        task     = "test"
        interval = "2s"
        timeout  = "2s"
        command  = "cat"
        args     = ["/tmp/${NOMAD_ALLOC_ID}-alive-2zb"]
      }
    }

    service {
      name = "service-3z"
      port = "9003"

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

    task "test" {
      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["-c", "sleep 15000"]
      }
    }
  }
}
