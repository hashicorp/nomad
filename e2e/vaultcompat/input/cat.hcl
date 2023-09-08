# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "cat" {
  type = "batch"
  group "testcase" {
    task "cat" {
      driver = "raw_exec"

      config {
        command = "cat"
        args    = ["${NOMAD_SECRETS_DIR}/vault_token"]
      }

      vault {
        policies = ["default"]
      }
    }

    restart {
      attempts = 0
      mode     = "fail"
    }
  }
}
