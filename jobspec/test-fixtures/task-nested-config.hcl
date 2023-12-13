# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


job "foo" {
  task "bar" {
    driver = "docker"

    config {
      image = "hashicorp/image"

      port_map {
        db = 1234
      }
    }
  }
}
