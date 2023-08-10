# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "huge" {
  type = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  meta {
    key = "REPLACE"
  }

  group "group" {
    task "task" {
      driver = "raw_exec"

      config {
        command = "/usr/bin/false"
      }

      resources {
        cpu    = 10
        memory = 16
      }
    }
  }
}
