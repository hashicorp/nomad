# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "address_mode_driver" {
  type = "service"

  group "group" {
    task "task" {
      service {
        name = "http-service"
        port = "http"

        address_mode = "auto"

        check {
          name = "http-check"
          type = "http"
          path = "/"
          port = "http"

          address_mode = "driver"
        }
      }

      service {
        name = "random-service"
        port = "9000"

        address_mode = "driver"

        check {
          name = "random-check"
          type = "tcp"
          port = "9001"

          address_mode = "driver"
        }
      }
    }
  }
}
