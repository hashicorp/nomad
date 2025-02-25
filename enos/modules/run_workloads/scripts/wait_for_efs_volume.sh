#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

# note: it can a very long time for CSI plugins and volumes to come up, and they
# are being created in parallel with this pre_start script
TIMEOUT=120
INTERVAL=2
last_error=
start_time=$(date +%s)

checkVolume() {
    local externalID mountTargetState
    nomad volume status efsTestVolume  || {
        last_error="could not find efsTestVolume"
        return 1
    }

    externalID=$(aws efs describe-file-systems | jq -r ".FileSystems[] | select(.Tags[0].Value == \"$VOLUME_TAG\")| .FileSystemId") || {
        last_error="Could not find volume for $VOLUME_TAG"
        return 1
    }

    # once the volume is created, it can take a while before the mount target
    # and its DNS name is available to plugins, which we need for mounting
    mountTargetState=$(aws efs describe-mount-targets --file-system-id "$externalID" | jq -r '.MountTargets[0].LifeCycleState')
    if [[ "$mountTargetState" == "available" ]]; then
        return 0
    fi

    last_error="mount target is not yet available"
    return 1
}

while :
do
    checkVolume && break

    current_time=$(date +%s)
    elapsed_time=$((current_time - start_time))
    if [ "$elapsed_time" -ge "$TIMEOUT" ]; then
        echo "Error: CSI volume did not become available within $TIMEOUT seconds: $last_error"
        exit 1
    fi

    sleep "$INTERVAL"
done
