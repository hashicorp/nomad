#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -xeuo pipefail

nomad volume status -type=host -verbose
nomad operator api /v1/nodes | jq '.[].HostVolumes'

addr="$(nomad service info -json job | jq -r '.[0].Address'):8000"
curl -sS "$addr/external/" | grep hi
curl -sS "$addr/internal/" | grep hi

echo '💚 looks good! 💚'
