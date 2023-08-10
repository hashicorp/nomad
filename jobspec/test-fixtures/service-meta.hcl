# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

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
