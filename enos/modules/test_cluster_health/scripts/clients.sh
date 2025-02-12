#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

error_exit() {
    printf 'Error: %s' "${1}" 
    exit 1
}

# Quality: "nomad_CLIENTS_status: A GET call to /v1/nodes returns the correct number of clients and they are all eligible and ready"

MAX_WAIT_TIME=20  # Maximum wait time in seconds
POLL_INTERVAL=2   # Interval between status checks

elapsed_time=0

while true; do
    clients_length=$(nomad node status -json | jq '[.[] | select(.Status == "ready")] | length')

    if [ "$clients_length" -eq "$CLIENT_COUNT" ]; then
        break
    fi

    if [ "$elapsed_time" -ge "$MAX_WAIT_TIME" ]; then
        error_exit "Unexpected number of ready clients: $clients_length"
    fi

    sleep "$POLL_INTERVAL"
    elapsed_time=$((elapsed_time + POLL_INTERVAL))
done

clients=$(nomad node status -json)
running_clients=$(echo "$clients" | jq '[.[] | select(.Status == "ready")]')

echo "$running_clients" | jq -c '.[]' | while read -r node; do
    status=$(echo "$node" | jq -r '.Status')
    eligibility=$(echo "$node" | jq -r '.SchedulingEligibility')

    if [ "$eligibility" != "eligible" ]; then
        error_exit "Client $(echo "$node" | jq -r '.Name') is not eligible!"
    fi
done

echo "All CLIENTS are eligible and running."
