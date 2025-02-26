#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

wget --header="Authorization: Bearer $TOKEN" -O "$LOCAL_ZIP" "$URL"

echo "File downloaded to $LOCAL_ZIP"

mkdir -p "$BINARY_PATH"
unzip -o "$LOCAL_ZIP" -d "$BINARY_PATH"

echo "File unzipped to $BINARY_PATH"

rm "$LOCAL_ZIP"
