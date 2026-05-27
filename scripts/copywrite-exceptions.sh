#!/bin/sh
# Copyright IBM Corp. 2015, 2026
# SPDX-License-Identifier: BUSL-1.1

# Patches up MPL-licensed subdirs by replacing the BUSL-license headers for MPL
# headers.

find . \
     -type f \( -name "*.go" -o -name "*.proto" \) \
     -exec sed -i '0,/BUSL-1.1/s//MPL-2.0/' {} \;
