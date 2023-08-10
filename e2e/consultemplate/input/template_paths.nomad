# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "template-paths" {
  datacenters = ["dc1", "dc2"]

  meta {
    ARTIFACT_DEST_DIR = "local/foo/src"
  }

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "template-paths" {

    task "task" {

      driver = "docker"

      config {
        image   = "busybox:1"
        command = "/bin/sh"
        args    = ["-c", "sleep 300"]
      }

      artifact {
        source      = "https://google.com"
        destination = "${NOMAD_META_ARTIFACT_DEST_DIR}"
      }

      template {
        source      = "${NOMAD_TASK_DIR}/foo/src"
        destination = "${NOMAD_SECRETS_DIR}/foo/dst"
      }

      template {
        destination = "${NOMAD_ALLOC_DIR}/shared.txt"
        data        = <<EOH
Data shared between all task in alloc dir.
EOH
      }

      resources {
        cpu    = 128
        memory = 64
      }
    }
  }
}
