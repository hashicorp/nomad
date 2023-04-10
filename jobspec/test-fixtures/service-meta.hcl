# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "service_meta" {
  type = "service"

  group "group" {
    task "task" {
      service {
        name = "http-service"

        meta {
          foo = "bar"
        }
      }
    }
  }
}
