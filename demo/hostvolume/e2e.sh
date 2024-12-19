#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -xeuo pipefail

./setup.sh
./check.sh
./teardown.sh
