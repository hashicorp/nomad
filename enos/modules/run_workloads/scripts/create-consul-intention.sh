#!/usr/bin/env bash
# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

dir=$(dirname "${BASH_SOURCE[0]}")
consul config write "${dir}/intention.hcl"
