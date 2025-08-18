# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "secret_value" {
  type        = string
  description = "The value of the randomly generated secret for this test"
}

job "custom_secret" {

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  update {
    min_healthy_time = "1s"
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
          TEST_ENV = "${var.secret_value}"
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
