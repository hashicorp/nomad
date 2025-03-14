#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

# Path enabled by the provision_cluster module: 
# https://github.com/hashicorp/nomad/e2e/terraform/provision-infra/hcp_vault.tf
secret_path="$VAULT_PATH/default/get-secret"

vault kv put "$secret_path" username="admin" password="supersecret"
