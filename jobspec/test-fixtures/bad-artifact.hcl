# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


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
