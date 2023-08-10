# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "bad-template-paths" {
  datacenters = ["dc1", "dc2"]

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "template-paths" {

    restart {
      attempts = 0
      mode     = "fail"
    }

    task "task" {

      driver = "docker"

      config {
        image   = "busybox:1"
        command = "/bin/sh"
        args    = ["-c", "sleep 300"]
      }

      template {
        source      = "/etc/passwd"
        destination = "${NOMAD_SECRETS_DIR}/foo/dst"
      }

      resources {
        cpu    = 128
        memory = 64
      }
    }
  }
}
