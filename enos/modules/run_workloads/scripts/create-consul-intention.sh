#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

dir=$(dirname "${BASH_SOURCE[0]}")
consul config write "${dir}/intention.hcl"
