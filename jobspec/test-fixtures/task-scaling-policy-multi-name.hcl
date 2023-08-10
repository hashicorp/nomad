# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "foo" {
  task "bar" {
    driver = "docker"

    scaling "cpu" "mem" {
      enabled = true
      min     = 50
      max     = 1000

      policy {
        test = "cpu"
      }
    }

  }
}

