#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

# when fingerprinted, the rocketduck plugin throws an exception if the NFS
# server isn't yet available and then never responds as ready to Probe attempts
# from the plugin supervisor

# note: it can a very long time for NFS to come up
TIMEOUT=120
INTERVAL=2
last_error=
start_time=$(date +%s)

checkNFSStatus() {
    local allocID status
    allocID=$(nomad job allocs -json nfs | jq -r '.[0].ID') || {
        last_error="could not query NFS alloc"
        return 1
    }

    status=$(nomad alloc checks -t '{{range .}}{{ printf "%s" .Status }}{{end}}' "$allocID") || {
        last_error="could not query NFS health checks"
        return 1
    }

    if [[ "$status" != "success" ]]; then
        last_error="expected NFS to be healthy, was $status"
        return 1
    fi
    return 0
}

while :
do
    checkNFSStatus && break

    current_time=$(date +%s)
    elapsed_time=$((current_time - start_time))
    if [ "$elapsed_time" -ge "$TIMEOUT" ]; then
        echo "Error: NFS did not become available within $TIMEOUT seconds: $last_error"
        exit 1
    fi

    sleep "$INTERVAL"
done
