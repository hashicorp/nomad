# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "cores-test" {
  group "group" {
    count = 5

    task "task" {
      driver = "docker"

      resources {
        cores  = 4
        memory = 128
      }
    }
  }
}
