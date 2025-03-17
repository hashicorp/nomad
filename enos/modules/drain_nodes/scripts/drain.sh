#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

error_exit() {
    printf 'Error: %s' "${1}"
    exit 1
}

DRAIN_DEADLINE="5s"

nodes=$(nomad node status -json | jq -r "[.[] | select(.Status == \"ready\") | .ID] | sort | .[:${NODES_TO_DRAIN}] | join(\" \")"  )

for node in $nodes; do
    echo "Draining the node $node"
    
    nomad node drain --enable --deadline "$DRAIN_DEADLINE" "$node" \
      || error_exit "Failed to drain node $node"

    allocs=$(nomad alloc status -json | jq --arg node "$node" '[.[] | select(.NodeID == $node and .ClientStatus == "running")] | length')
    if [ $? -ne 0 ]; then
        error_exit "Allocs still running on $node"
    fi

    nomad node drain --disable  "$node" \
      || error_exit "Failed to disable drain for node $node"
    
    nomad node eligibility -enable  "$node" \
      || error_exit "Failed to set node $node back to eligible"
done
