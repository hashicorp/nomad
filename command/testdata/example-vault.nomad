# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "vault" {
  datacenters = ["dc1"]
  group "group" {
    task "task" {
      driver = "docker"
      config {
        image = "alpine:latest"
      }
      vault {
        policies = ["my-policy"]
      }
    }
  }
}
