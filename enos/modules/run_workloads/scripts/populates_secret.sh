#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

vault kv put "$VAULT_PATH/default/get-secret" username="admin" password="supersecret"
