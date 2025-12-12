#!/usr/bin/env bash
# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

error_exit() {
    printf 'Error: %s' "${1}"
    echo "Allocs on node ${client_id}:"
    nomad alloc status -json | \
        jq -r --arg client_id "$client_id" '[.[] | select(.NodeID == $client_id)]'
    exit 1
}

MAX_WAIT_TIME=60  # Maximum wait time in seconds
POLL_INTERVAL=2   # Interval between status checks

elapsed_time=0
last_error=
client_id=

checkClientReady() {
    local client client_status
    echo "Checking client health for $CLIENT_IP"

    client=$(nomad node status -address "https://$CLIENT_IP:4646" -self -json) || {
        last_error="Unable to get info for node at $CLIENT_IP"
        return 1
    }
    client_status=$(echo "$client" | jq  -r '.Status')
    if [ "$client_status" == "ready" ]; then
        client_id=$(echo "$client" | jq '.ID' | tr -d '"')
        last_error=
        return 0
    fi

    last_error="Node at $CLIENT_IP is ${client_status}, not ready"
    return 1
}

while true; do
    checkClientReady && break
    if [ "$elapsed_time" -ge "$MAX_WAIT_TIME" ]; then
        error_exit "$last_error within $elapsed_time seconds."
    fi

    echo "$last_error within $elapsed_time seconds. Retrying in $POLL_INTERVAL seconds..."
    sleep "$POLL_INTERVAL"
    elapsed_time=$((elapsed_time + POLL_INTERVAL))
done

echo "Client $client_id at $CLIENT_IP is ready"

allocs_count=$(echo $ALLOCS | jq '[ .[] | select(.ClientStatus == "running")] | length')
echo "$allocs_count allocs found before upgrade $ALLOCS"

# Quality: "nomad_alloc_reconnect: A GET call to /v1/allocs will return the same IDs for running allocs before and after a client upgrade on each client"

checkAllocsCount() {
    running_allocs=$(nomad alloc status -json | jq -r --arg client_id "$client_id" '[.[] | select(.ClientStatus == "running" and .NodeID == $client_id)]') || {
        last_error="Failed to check alloc status"
        return 1
    }
    allocs_length=$(echo "$running_allocs" | jq 'length') \
        || error_exit "Invalid alloc status -json output"

    if [ "$allocs_length" -eq "$allocs_count" ]; then
        return 0
    fi

    last_error="Some allocs are not running"
    return 1
}

echo "Reading allocs for client at $CLIENT_IP"

elapsed_time=0
while true; do
    checkAllocsCount && break

    if [ "$elapsed_time" -ge "$MAX_WAIT_TIME" ]; then
        error_exit "$last_error within $elapsed_time seconds."
    fi

    echo "Running allocs: $allocs_length, expected ${allocs_count}. Have been waiting for ${elapsed_time}. Retrying in $POLL_INTERVAL seconds..."
    sleep $POLL_INTERVAL
    elapsed_time=$((elapsed_time + POLL_INTERVAL))

done

echo "Correct number of allocs found running: $allocs_length"

current_allocs=$(nomad alloc status -json | jq -r --arg client_id "$client_id" '[.[] | select(.ClientStatus == "running" and .NodeID == $client_id) | .ID] | join(" ")')
if [ -z "$current_allocs" ]; then
    error_exit "Failed to read allocs for node: $client_id"
fi

IDs=$(echo $ALLOCS | jq -r '[ .[] | select(.ClientStatus == "running")] | [.[].ID] | join(" ")')

IFS=' ' read -r -a INPUT_ARRAY <<< "${IDs[*]}"
IFS=' ' read -r -a RUNNING_ARRAY <<< "$current_allocs"

sorted_input=($(printf "%s\n" "${INPUT_ARRAY[@]}" | sort))
sorted_running=($(printf "%s\n" "${RUNNING_ARRAY[@]}" | sort))

if [[ "${sorted_input[*]}" != "${sorted_running[*]}" ]]; then
    error_exit "Different allocs found, expected: ${sorted_input[*]} found: ${sorted_running[*]}"
fi

echo "All allocs reattached correctly for node at $CLIENT_IP"
