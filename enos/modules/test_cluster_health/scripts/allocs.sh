#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

error_exit() {
    printf 'Error: %s' "${1}" 
    exit 1
}

# Quality: nomad_allocs_status: A GET call to /v1/allocs returns the correct number of allocations and they are all running

allocs=$(nomad alloc status -json)
running_allocs=$(echo $allocs | jq '[.[] | select(.ClientStatus == "running")]')
allocs_length=$(echo "$running_allocs" | jq 'length' )

if [ -z "$allocs_length" ];  then
    error_exit "No allocs found"
fi

if [ "$allocs_length" -ne "$ALLOC_COUNT" ]; then
    error_exit "Some allocs are not running:\n$(nomad alloc status -json | jq -r '.[] | select(.ClientStatus != "running") | .ID')"
fi

echo "All allocs are running."

# Quality: nomad_reschedule_alloc: A POST / PUT call to /v1/allocation/:alloc_id/stop results in the stopped allocation being rescheduled

MAX_WAIT_TIME=30  # Maximum wait time in seconds
POLL_INTERVAL=2    # Interval between status checks

random_alloc_id=$(echo "$running_allocs" | jq -r ".[$((RANDOM % ($allocs_length + 1)))].ID")
nomad alloc stop -detach "$random_alloc_id" || error_exit "Failed to stop allocation $random_alloc_id."

echo "Waiting for allocation $random_alloc_id to reach 'complete' status..."
elapsed_time=0
while alloc_status=$(nomad alloc status -json "$random_alloc_id" | jq -r '.ClientStatus'); [ "$alloc_status" != "complete" ]; do
    if [ "$elapsed_time" -ge "$MAX_WAIT_TIME" ]; then
        echo "Error: Allocation $random_alloc_id did not reach 'complete' status within $MAX_WAIT_TIME seconds."
        exit 1
    fi

    echo "Current status: $alloc_status. Retrying in $POLL_INTERVAL seconds..."
    sleep $POLL_INTERVAL
    elapsed_time=$((elapsed_time + POLL_INTERVAL))
done

echo "Waiting for all the allocations to be running again"
elapsed_time=0
while new_allocs=$(nomad alloc status -json | jq '[.[] | select(.ClientStatus == "running")]'); [ $(echo "$new_allocs" | jq 'length') != "$ALLOCS" ]; do
    if [ "$elapsed_time" -ge "$MAX_WAIT_TIME" ]; then
        echo "Error: Allocation $random_alloc_id did not reach 'complete' status within $MAX_WAIT_TIME seconds."
        exit 1
    fi

    echo "Current status: $alloc_status. Retrying in $POLL_INTERVAL seconds..."
    sleep $POLL_INTERVAL
    elapsed_time=$((elapsed_time + POLL_INTERVAL))
done

echo "Alloc successfully restarted"
