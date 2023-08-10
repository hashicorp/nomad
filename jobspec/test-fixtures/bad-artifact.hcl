# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "binstore-storagelocker" {
  group "binsl" {
    count = 5

    task "binstore" {
      driver = "docker"

      artifact {
        bad = "bad"
      }

      resources {}
    }
  }
}
