# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

job "cat_jwt" {
  type = "batch"

  // Tasks in this group are expected to succeed and run to completion.
  group "success" {
    vault {}

    // Task default_identity uses the default workload identity injected by the
    // server and the inherits the Vault configuration from the group.
    task "default_identity" {
      driver = "raw_exec"

      config {
        command = "cat"
        args    = ["${NOMAD_SECRETS_DIR}/secret.txt"]
      }

      template {
        data        = <<EOF
{{with secret "secret/data/default/cat_jwt"}}{{.Data.data.secret}}{{end}}
EOF
        destination = "${NOMAD_SECRETS_DIR}/secret.txt"
      }
    }

    // Task custom_identity uses a custom workload identity configuration for
    // Vault that exposes the JWT as a file and expand on the group Vault
    // configuration.
    task "custom_identity" {
      driver = "raw_exec"

      config {
        command = "cat"
        args = [
          "${NOMAD_SECRETS_DIR}/secret.txt",
          "${NOMAD_SECRETS_DIR}/nomad_vault_default.jwt",
        ]
      }

      template {
        data        = <<EOF
{{with secret "secret/data/restricted"}}{{.Data.data.secret}}{{end}}
EOF
        destination = "${NOMAD_SECRETS_DIR}/secret.txt"
      }

      vault {
        role = "nomad-restricted"
      }

      identity {
        name = "vault_default"
        aud  = ["vault.io"]
        ttl  = "10m"
        file = true
      }
    }

    restart {
      attempts = 0
      mode     = "fail"
    }
  }

  // Tasks in this group are expected to fail or never complete.
  group "fail" {

    // Task unauthorized fails to access secrets it doesn't have access to.
    task "unauthorized" {
      driver = "raw_exec"

      config {
        command = "cat"
        args    = ["${NOMAD_SECRETS_DIR}/secret.txt"]
      }

      template {
        data        = <<EOF
{{with secret "secret/data/restricted"}}{{.Data.data.secret}}{{end}}
EOF
        destination = "${NOMAD_SECRETS_DIR}/secret.txt"
      }

      vault {}
    }

    // Task missing_vault fails to access the Vault token because it doesn't
    // have a vault block, so Nomad doesn't derive a token.
    task "missing_vault" {
      driver = "raw_exec"

      config {
        command = "cat"
        args    = ["${NOMAD_SECRETS_DIR}/vault_token"]
      }
    }

    restart {
      attempts = 0
      mode     = "fail"
    }

    reschedule {
      attempts  = 0
      unlimited = false
    }
  }
}
