# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "vault_secret" {

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

      vault {}

      secret "testsecret" {
        provider = "vault"
        path     = "SECRET_PATH"
        config {
          engine = "kv_v2"
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
