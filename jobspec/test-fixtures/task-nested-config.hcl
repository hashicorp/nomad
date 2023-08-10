# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

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
