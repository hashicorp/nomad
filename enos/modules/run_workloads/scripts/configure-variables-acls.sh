#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

nomad acl policy apply \
   -namespace default -job writes-vars \
   writes-vars-policy - <<EOF
namespace "default" {
  variables {
    path "nomad/jobs/writes-vars" {
      capabilities = ["write", "read"]
    }
  }
}
EOF
