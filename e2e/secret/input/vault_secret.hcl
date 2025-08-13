# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "secret_path" {
  type        = string
  description = "The path of the vault secret"
}

job "vault_secret" {

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

      vault {}

      secret "testsecret" {
        provider = "vault"
        path     = "${var.secret_path}"
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
