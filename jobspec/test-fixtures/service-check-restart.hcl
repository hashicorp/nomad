# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "service_check_restart" {
  type = "service"

  group "group" {
    task "task" {
      service {
        name = "http-service"

        check_restart {
          limit           = 3
          grace           = "10s"
          ignore_warnings = true
        }

        check {
          name = "random-check"
          type = "tcp"
          port = "9001"
        }
      }
    }
  }
}
