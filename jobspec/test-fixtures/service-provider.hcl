# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "service-provider" {
  group "group" {
    count = 5

    task "task" {
      driver = "docker"

      service {
        name     = "service-provider"
        provider = "nomad"
      }
    }
  }
}
