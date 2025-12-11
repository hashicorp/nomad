#!/usr/bin/env bash
# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

# note: it can a very long time for CSI plugins and volumes to come up, and they
# are being created in parallel with this pre_start script
TIMEOUT=120
INTERVAL=2
last_error=
start_time=$(date +%s)

checkPlugin() {
    local pluginStatus foundNodes foundControllers
    pluginStatus=$(nomad plugin status rocketduck-nfs) || {
        last_error="could not read CSI plugin status"
        return 1
    }

    foundControllers=$(echo "$pluginStatus" | awk -F'= +' '/Controllers Healthy/{print $2}')
    if [[ "$foundControllers" != 1 ]]; then
        last_error="expected plugin to have 1 healthy controller, found $foundControllers"
        return 1
    fi


    foundNodes=$(echo "$pluginStatus" | awk -F'= +' '/Nodes Healthy/{print $2}')
    if [[ "$foundNodes" == 0 ]]; then
        last_error="expected plugin to have at least 1 healthy nodes, found none"
        return 1
    fi
    last_error=
    return 0
}

createVolume() {
    dir=$(dirname "${BASH_SOURCE[0]}")
    nomad volume create "${dir}/nfs-volume.hcl" || {
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

createVolume && echo "Created volume"
