# Copyright IBM Corp. 2015, 2026
# SPDX-License-Identifier: BUSL-1.1

path "secrets-TESTID/data/myapp" {
  capabilities = ["read"]
}

path "pki-TESTID/issue/nomad" {
  capabilities = ["create", "update", "read"]
}
