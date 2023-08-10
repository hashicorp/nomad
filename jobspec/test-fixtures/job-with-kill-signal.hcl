# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "foo" {
  task "bar" {
    driver      = "docker"
    kill_signal = "SIGQUIT"

    config {
      image = "hashicorp/image"
    }
  }
}
