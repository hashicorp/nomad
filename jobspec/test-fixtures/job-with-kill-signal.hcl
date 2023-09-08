# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "foo" {
  task "bar" {
    driver      = "docker"
    kill_signal = "SIGQUIT"

    config {
      image = "hashicorp/image"
    }
  }
}
