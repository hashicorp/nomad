#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

error_exit() {
    printf 'Error: %s' "${1}" 
    exit 1
}

MAX_WAIT_TIME=40
POLL_INTERVAL=2

elapsed_time=0

# Quality: nomad_allocs_status: A GET call to /v1/allocs returns the correct number of allocations and they are all running

while true; do    
    allocs=$(nomad alloc status -json)
    if [ $? -ne 0 ]; then
        error_exit "Error running 'nomad alloc status': $allocs"
    fi

    running_allocs=$(echo $allocs | jq '[.[] | select(.ClientStatus == "running")]')
    allocs_length=$(echo $running_allocs | jq 'length')
    if [ -z "$allocs_length" ];  then
        error_exit "No allocs found"
    fi

    if [ "$allocs_length" -eq "$ALLOC_COUNT" ]; then
       break
    fi

    if [ "$elapsed_time" -ge "$MAX_WAIT_TIME" ]; then
        error_exit "Some allocs are not running:\n$(nomad alloc status -json | jq -r '.[] | select(.ClientStatus != "running") | .ID')"   error_exit "Unexpected number of ready clients: $clients_length"
    fi

    echo "Running allocs: $$running_allocs, expected "$ALLOC_COUNT". Waiting for $elapsed_time  Retrying in $POLL_INTERVAL seconds..."
    sleep $POLL_INTERVAL
    elapsed_time=$((elapsed_time + POLL_INTERVAL))
done

echo "All ALLOCS are running."

# Quality: nomad_reschedule_alloc: A POST / PUT call to /v1/allocation/:alloc_id/stop results in the stopped allocation being rescheduled

random_index=$((RANDOM % allocs_length))
random_alloc_id=$(echo "$running_allocs" | jq -r ".[${random_index}].ID")

error_ms=$(nomad alloc stop "$random_alloc_id" 2>&1)
if [ $? -ne 0 ]; then
    error_exit "Failed to stop allocation $random_alloc_id. Error: $error_msg"
fi

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
    new_allocs=$(nomad alloc status -json | jq '[.[] | select(.ClientStatus == "running")]')
    running_new_allocs=$(echo "$new_allocs" | jq 'length')
    
    if [ "$running_new_allocs" == "$ALLOC_COUNT" ]; then
        break
    fi
    
    if [ "$elapsed_time" -ge "$MAX_WAIT_TIME" ]; then
        error_exit "Expected $ALLOC_COUNT running allocations, found $running_new_allocs after $elapsed_time seconds"
    fi

    echo "Expected $ALLOC_COUNT running allocations, found $running_new_allocs Retrying in $POLL_INTERVAL seconds..."
    sleep $POLL_INTERVAL
    elapsed_time=$((elapsed_time + POLL_INTERVAL))
done

echo "Alloc successfully rescheduled"
