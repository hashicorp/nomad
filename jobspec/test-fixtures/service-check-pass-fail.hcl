# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "check_pass_fail" {
  type = "service"

  group "group" {
    count = 1

    task "task" {
      service {
        name = "service"
        port = "http"

        check {
          name                     = "check-name"
          type                     = "http"
          path                     = "/"
          method                   = "POST"
          interval                 = "10s"
          timeout                  = "2s"
          initial_status           = "passing"
          success_before_passing   = 3
          failures_before_critical = 4
        }
      }
    }
  }
}
