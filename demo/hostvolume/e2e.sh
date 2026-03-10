#!/usr/bin/env bash
# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

set -xeuo pipefail

./setup.sh
./check.sh
./teardown.sh
