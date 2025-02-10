# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "restricted_jwt" {
  type = "batch"

  // Tasks in this group are expected to succeed and run to completion.
  group "success" {
    vault {}

    count = 2

    // Task default_identity uses the default workload identity injected by the
    // server and the inherits the Vault configuration from the group.
    task "authorized" {
      driver = "raw_exec"

      config {
        command = "cat"
        args    = ["${NOMAD_SECRETS_DIR}/secret.txt"]
      }

      // Vault has an alias that maps this job's nomad_workload_id to an entity
      // with a policy that allows access to these secrets
      template {
        data        = <<EOF
{{with secret "secret/data/restricted"}}{{.Data.data.secret}}{{end}}
EOF
        destination = "${NOMAD_SECRETS_DIR}/secret.txt"
      }

      restart {
        attempts = 0
        mode     = "fail"
      }
    }
  }
}
