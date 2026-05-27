#!/usr/bin/env bash
# Copyright IBM Corp. 2015, 2025
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

    # we --ignore-system both to exercise the feature and make sure we won't
    # have to reschedule system jobs and wait for them again
    nomad node drain --enable --ignore-system --deadline "$DRAIN_DEADLINE" "$node" \
      || error_exit "Failed to drain node $node"

    allocs=$(nomad alloc status -json | jq --arg node "$node" '[.[] | select(.NodeID == $node and .ClientStatus == "running")] | length')
    if [ $? -ne 0 ]; then
        error_exit "Allocs still running on $node"
    fi

    nomad node drain --disable  "$node" \
      || error_exit "Failed to disable drain for node $node"
done
