# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "group_check" {
  datacenters = ["dc1"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group_check" {
    network {
      mode = "bridge"
    }

    service {
      name = "group-service-1"
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
      name = "group-service-2"
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
        name     = "alive-2b"
        type     = "script"
        task     = "test"
        interval = "2s"
        timeout  = "2s"
        command  = "cat"
        args     = ["/tmp/${NOMAD_ALLOC_ID}-alive-2b"]
      }
    }

    service {
      name = "group-service-3"
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

    count = 1

    task "test" {
      driver = "raw_exec"

      config {
        command = "bash"
        args    = ["-c", "sleep 15000"]
      }
    }
  }
}
