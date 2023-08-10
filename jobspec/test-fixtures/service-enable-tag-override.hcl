# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "service_eto" {
  type = "service"

  group "group" {
    task "task" {
      service {
        name                = "example"
        enable_tag_override = true
      }
    }
  }
}
