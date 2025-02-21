#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

error_exit() {
    printf 'Error: %s' "${1}"
    exit 1
}

MAX_WAIT_TIME=120
POLL_INTERVAL=2

elapsed_time=0

# Quality: nomad_allocs_status: A GET call to /v1/allocs returns the correct number of allocations and they are all running

running_allocs=
allocs_length=

checkAllocsCount() {
    local allocs
    allocs=$(nomad alloc status -json) || error_exit "Failed to check alloc status"

    running_allocs=$(echo "$allocs" | jq '[.[] | select(.ClientStatus == "running")]')
    allocs_length=$(echo "$running_allocs" | jq 'length') \
        || error_exit "Invalid alloc status -json output"

    if [ "$allocs_length" -eq "$ALLOC_COUNT" ]; then
        return 0
    fi

    return 1
}

while true; do
    checkAllocsCount && break

    if [ "$elapsed_time" -ge "$MAX_WAIT_TIME" ]; then
        error_exit "Some allocs are not running:\n$(nomad alloc status -json | jq -r '.[] | select(.ClientStatus != "running") | .ID')"
    fi

    echo "Running allocs: $running_allocs, expected $ALLOC_COUNT. Waiting for $elapsed_time  Retrying in $POLL_INTERVAL seconds..."
    sleep $POLL_INTERVAL
    elapsed_time=$((elapsed_time + POLL_INTERVAL))
done

echo "All ALLOCS are running."

if [ "$allocs_length" -eq 0 ]; then
    exit 0
fi

# Quality: nomad_reschedule_alloc: A POST / PUT call to /v1/allocation/:alloc_id/stop results in the stopped allocation being rescheduled

random_index=$((RANDOM % allocs_length))
random_alloc_id=$(echo "$running_allocs" | jq -r ".[${random_index}].ID")

nomad alloc stop "$random_alloc_id" \
    || error_exit "Failed to stop allocation $random_alloc_id"

echo "Waiting for allocation $random_alloc_id to reach 'complete' status..."
elapsed_time=0

while true; do
    alloc_status=$(nomad alloc status -json "$random_alloc_id" | jq -r '.ClientStatus')
    if [ "$alloc_status" == "complete" ]; then
        break
    fi

    if [ "$elapsed_time" -ge "$MAX_WAIT_TIME" ]; then
        error_exit "Allocation $random_alloc_id did not reach 'complete' status within $MAX_WAIT_TIME seconds."
    fi

    echo "Current status: $alloc_status, not 'complete'. Waiting for $elapsed_time  Retrying in $POLL_INTERVAL seconds..."
    sleep $POLL_INTERVAL
    elapsed_time=$((elapsed_time + POLL_INTERVAL))
done

echo "Waiting for all the allocations to be running again"
elapsed_time=0

while true; do
    # reset
    running_allocs=
    allocs_length=

    checkAllocsCount && break

    if [ "$elapsed_time" -ge "$MAX_WAIT_TIME" ]; then
        error_exit "Expected $ALLOC_COUNT running allocations, found $running_allocs after $elapsed_time seconds"
    fi

    echo "Expected $ALLOC_COUNT running allocations, found $running_allocs Retrying in $POLL_INTERVAL seconds..."
    sleep $POLL_INTERVAL
    elapsed_time=$((elapsed_time + POLL_INTERVAL))
done

echo "Alloc successfully rescheduled"
