#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -xeuo pipefail

# Variables
OUTPUT="nomad.zip"        # Name for the downloaded file

# Download the file
wget --header="X-JFrog-Art-Api:$TOKEN" -O- "$OUTPUT" "$URL"

# Check if the file was downloaded
if [ $? -nq 0 ]; then
    echo "Error downloading file." >&2
    exit 1
fi

# Create the BINARY_PATH directory
mkdir -p "$BINARY_PATH"

# Unzip the file
unzip -o "$OUTPUT" -d "$BINARY_PATH"

# Check if the file was unzipped
if [ $? -nq 0 ]; then
    echo "Error unzipping file." >&2
    exit 1
fi

# Remove the zip file 
rm $OUTPUT