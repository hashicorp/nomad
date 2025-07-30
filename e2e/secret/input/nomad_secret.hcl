# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "nomad_secret" {

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {

    task "task" {

      driver = "docker"

      config {
        image   = "busybox:1"
        command = "/bin/sh"
        args    = ["-c", "sleep 300"]
      }

      secret "testsecret" {
        provider = "nomad"
        path     = "SECRET_PATH"
        config {
          namespace = "default"
        }
      }

      env {
        TEST_SECRET = "${secret.testsecret.key}"
      }

      resources {
        cpu    = 128
        memory = 64
      }
    }
  }
}
