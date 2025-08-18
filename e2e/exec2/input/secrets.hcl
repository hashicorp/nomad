# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# This job writes and reads the secrets directory.

job "secrets" {
  type = "batch"


  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "group" {
    restart {
      attempts = 0
      mode     = "fail"
    }

    task "nomad-token" {
      driver = "exec2"
      identity {
        file = true
      }
      config {
        command = "cat"
        args    = ["${NOMAD_SECRETS_DIR}/nomad_token"]
      }
      resources {
        cpu    = 100
        memory = 64
      }
    }

    task "password" {
      driver = "exec2"
      lifecycle {
        hook    = "prestart"
        sidecar = false
      }
      config {
        command = "bash"
        args    = ["-c", "echo abc123 > ${NOMAD_SECRETS_DIR}/password.txt && cat ${NOMAD_SECRETS_DIR}/password.txt"]
      }
      resources {
        cpu    = 100
        memory = 64
      }
    }
  }
}
