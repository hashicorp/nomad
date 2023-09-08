# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "example" {
  group "group" {
    task "task" {
      template {
        wait {
          min = "5s"
          max = "60s"
        }
      }
    }
  }
}
