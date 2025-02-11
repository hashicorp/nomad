#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

error_exit() {
    printf 'Error: %s' "${1}" 
    exit 1
}


timestamp=$(date +"%Y-%m-%d_%H-%M-%S")
echo $SERVER
nomad operator autopilot health -json
nomad operator snapshot save -stale -address https://$SERVER:4646 $timestamp-2.snap
