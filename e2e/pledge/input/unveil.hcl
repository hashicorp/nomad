# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "unveil" {
  type = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {

    reschedule {
      attempts  = 0
      unlimited = false
    }

    restart {
      attempts = 0
      mode     = "fail"
    }

    task "cat" {
      driver = "pledge"
      config {
        command  = "cat"
        args     = ["/etc/passwd"]
        promises = "stdio rpath"
        unveil   = ["r:/etc/passwd"]
      }
    }
  }
}
