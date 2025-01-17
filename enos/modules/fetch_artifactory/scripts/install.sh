#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -xeuo pipefail

# Variables
LOCAL_ZIP="nomad.zip"        # Name for the downloaded file

# Download the file
wget --header="X-JFrog-Art-Api:$TOKEN" -O "$LOCAL_ZIP" "$URL"

#Check if the file was downloaded
if [ $? -eq 0 ]; then
    echo "File downloaded successfully: $LOCAL_ZIP"
else
    echo "Error downloading file." >&2
    exit 1
fi

# Create the BINARY_PATH directory
mkdir -p "$BINARY_PATH"

# Unzip the file
unzip -o "$LOCAL_ZIP" -d "$BINARY_PATH"

# Check if the file was unzipped
if [ $? -eq 0 ]; then
    echo "File unzipped successfully to $BINARY_PATH"
else
    echo "Error unzipping file." >&2
    exit 1
fi

# Remove the zipped file
rm "$LOCAL_ZIP"