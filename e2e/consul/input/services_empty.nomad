# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "consul-register-on-update" {
  datacenters = ["dc1"]
  type        = "service"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "echo" {

    task "busybox-nc" {
      driver = "docker"

      config {
        image   = "busybox:1"
        command = "nc"
        args = [
          "-ll",
          "-p",
          "1234",
          "-e",
        "/bin/cat"]
      }

      # no initial service definition
    }
  }
}