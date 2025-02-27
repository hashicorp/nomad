#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

# note: it can a very long time for plugins to come up
TIMEOUT=60
INTERVAL=2
last_error=
start_time=$(date +%s)

checkPlugin() {
    local pluginStatus foundNodes
    pluginStatus=$(nomad plugin status aws-efs0) || {
        last_error="could not read CSI plugin status"
        return 1
    }

    foundNodes=$(echo "$pluginStatus" | awk -F'= +' '/Nodes Healthy/{print $2}')
    if [[ "$foundNodes" == 0 ]]; then
        last_error="expected plugin to have at least 1 healthy nodes, found none"
        return 1
    fi
    return 0
}

registerVolume() {
    local externalID idempotencyToken dir
    idempotencyToken=$(uuidgen)
    dir=$(dirname "${BASH_SOURCE[0]}")
    externalID=$(aws efs describe-file-systems | jq -r ".FileSystems[] | select(.Tags[0].Value == \"$VOLUME_TAG\")| .FileSystemId") || {
        echo "Could not find volume for $VOLUME_TAG"
        exit 1
    }

    sed -e "s/IDEMPOTENCY_TOKEN/$idempotencyToken/" \
        -e "s/EXTERNAL_ID/$externalID/" \
        "${dir}/volume.hcl.tpl" | nomad volume register - || {
        echo "Could not register volume"
        exit 1
    }
}

while :
do
    checkPlugin && break

    current_time=$(date +%s)
    elapsed_time=$((current_time - start_time))
    if [ "$elapsed_time" -ge "$TIMEOUT" ]; then
        echo "Error: CSI plugin did not become available within $TIMEOUT seconds: $last_error"
        exit 1
    fi

    sleep "$INTERVAL"
done

registerVolume
nomad volume status -type csi
