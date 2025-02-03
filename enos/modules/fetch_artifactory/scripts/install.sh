#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -xeuo pipefail

wget --header="X-JFrog-Art-Api:$TOKEN" -O "$LOCAL_ZIP" "$URL"

if [ $? -eq 0 ]; then
    echo "File downloaded successfully: $LOCAL_ZIP"
else
    echo "Error downloading file." >&2
    exit 1
fi

mkdir -p "$BINARY_PATH"
unzip -o "$LOCAL_ZIP" -d "$BINARY_PATH"

if [ $? -eq 0 ]; then
    echo "File unzipped successfully to $BINARY_PATH"
else
    echo "Error unzipping file." >&2
    exit 1
fi

rm "$LOCAL_ZIP"
