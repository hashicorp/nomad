#!/usr/bin/env bash
# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

error_exit() {
    printf 'Error: %s' "${1}"
    echo "All allocs:"
    nomad alloc status -json
    exit 1
}

MAX_WAIT_TIME=120
POLL_INTERVAL=2

elapsed_time=0

# Quality: nomad_allocs_status: A GET call to /v1/allocs returns the correct number of allocations and they are all running

running_allocs=
allocs_length=
last_error=

checkAllocsCount() {
    local allocs
    allocs=$(nomad alloc status -json) || {
        last_error="Failed to check alloc status"
        return 1
    }

    running_allocs=$(echo "$allocs" | jq '[.[] | select(.ClientStatus == "running")]')
    allocs_length=$(echo "$running_allocs" | jq 'length') \
        || error_exit "Invalid alloc status -json output"

    if [ "$allocs_length" -eq "$ALLOC_COUNT" ]; then
        return 0
    fi

    last_error="Some allocs are not running"
    return 1
}

while true; do
    checkAllocsCount && break

    if [ "$elapsed_time" -ge "$MAX_WAIT_TIME" ]; then
        error_exit "$last_error within $elapsed_time seconds."
    fi

    echo "Running allocs: $allocs_length, expected ${ALLOC_COUNT}. Have been waiting for ${elapsed_time}. Retrying in $POLL_INTERVAL seconds..."
    sleep $POLL_INTERVAL
    elapsed_time=$((elapsed_time + POLL_INTERVAL))
done

echo "All $ALLOC_COUNT ALLOCS are running."

if [ "$allocs_length" -eq 0 ]; then
    exit 0
fi

# Quality: nomad_reschedule_alloc: A POST / PUT call to /v1/allocation/:alloc_id/stop results in the stopped allocation being rescheduled

service_batch_allocs=$(echo "$running_allocs" | jq  '[.[] |select(.JobType != "system")]')
service_batch_allocs_length=$(echo "$service_batch_allocs" | jq 'length' )
random_index=$((RANDOM % service_batch_allocs_length))
random_alloc_id=$(echo "$service_batch_allocs" | jq -r ".[${random_index}].ID")

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
        nomad alloc status -json > allocs.json
        error_exit "Expected $ALLOC_COUNT running allocations, found $allocs_length after $elapsed_time seconds"
    fi

    echo "Expected $ALLOC_COUNT running allocations, found $allocs_length Retrying in $POLL_INTERVAL seconds..."
    sleep $POLL_INTERVAL
    elapsed_time=$((elapsed_time + POLL_INTERVAL))
done

echo "Alloc successfully rescheduled"
