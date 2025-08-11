# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "custom_secret" {

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
        provider = "test_secret_plugin"
        path     = "some/path"
        env {
          // The custom plugin will output this as part of the result field
          TEST_ENV = "SECRET_VALUE"
        }
      }

      env {
        TEST_SECRET = "${secret.testsecret.TEST_ENV}"
      }

      resources {
        cpu    = 128
        memory = 64
      }
    }
  }
}
