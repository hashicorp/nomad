# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "address" {
  type        = string
  description = "The address to cURL"
}

job "curl" {
  type = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {
    network {
      mode = "host"
    }

    reschedule {
      attempts  = 0
      unlimited = false
    }

    restart {
      attempts = 0
      mode     = "fail"
    }

    task "curl" {
      driver = "pledge"
      config {
        command  = "curl"
        args     = ["${var.address}"]
        promises = "stdio rpath inet dns sendfd"
      }
    }
  }
}
