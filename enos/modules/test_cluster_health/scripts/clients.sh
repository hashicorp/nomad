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
ready_clients=
last_error=

checkReadyClients() {
    local clients_length

    ready_clients=$(nomad node status -json | jq '[.[] | select(.Status == "ready")]') ||
        error_exit "Could not query node status"

    clients_length=$(echo "$ready_clients" | jq 'length')
    if [ "$clients_length" -eq "$CLIENT_COUNT" ]; then
        last_error=
        return 0
    fi

    last_error="Unexpected number of ready clients: $clients_length"
    return 1
}

checkEligibleClients() {
    echo "$ready_clients" | jq -e '
        map(select(.SchedulingEligibility != "eligible")) | length == 0' && return 0

    last_error=$(echo "$ready_clients" | jq -r '
        map(select(.SchedulingEligibility != "eligible")) | "\(.[].ID) is ineligible"')
    return 1
}

while true; do
    checkReadyClients && checkEligibleClients && break

    if [ "$elapsed_time" -ge "$MAX_WAIT_TIME" ]; then
        error_exit "$last_error"
    fi

    sleep "$POLL_INTERVAL"
    elapsed_time=$((elapsed_time + POLL_INTERVAL))
done

echo "All clients are eligible and running."
