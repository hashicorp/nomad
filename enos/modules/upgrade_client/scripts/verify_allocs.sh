#!/usr/bin/env bash
# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

error_exit() {
    printf 'Error: %s' "${1}"
    ALL_ALLOCS=$(nomad alloc status -json | \
                     jq -r --arg client_id "$client_id" '[.[] | select(.NodeID == $client_id)]')
    mkdir -p /tmp/artifacts
    OUT="/tmp/artifacts/allocs.json"
    echo "$ALL_ALLOCS" > "$OUT"
    echo
    echo "Expected allocs:"
    echo "$ALLOCS"
    echo
    echo "Allocs on node ${client_id}:"
    cat "$OUT" | jq -r '
        ["ID", "Node", "ClientStatus", "DesiredStatus", "JobID"],
        ["--------", "--------", "------------", "-------------", "---------------"],
        (.[] | [.ID[:8], .NodeID[:8], .ClientStatus, .DesiredStatus, .JobID])
        | @tsv' | column -ts $'\t'

    echo "full allocation status for debugging written to: $OUT"
    exit 1
}

MAX_WAIT_TIME=60  # Maximum wait time in seconds
POLL_INTERVAL=2   # Interval between status checks

IFS=',' read -r -a ALLOCS <<< "$ALLOCS"

# we'll collect a list of allocs that aren't running
declare -A MISSING_ALLOCS

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

# checks that each allocation is still running
checkAllocations() {
    local alloc
    local ok
    ok=0
    MISSING_ALLOCS=()

    for alloc in "${ALLOCS[@]}"; do
        status=$(nomad alloc status -json "$alloc" | jq -r '.ClientStatus')
        if [[ "$status" != "running" ]]; then
            MISSING_ALLOCS["$alloc"]=1
            last_error="Some allocs were not running: ${!MISSING_ALLOCS[*]}"
            ok=1
        fi
    done

    return "$ok"
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

# Quality: "nomad_alloc_reconnect: A GET call to /v1/allocs will return the same IDs for running allocs before and after a client upgrade on each client"

echo "Reading allocs for client at $CLIENT_IP"

elapsed_time=0
while true; do
    checkAllocations && break

    if [ "$elapsed_time" -ge "$MAX_WAIT_TIME" ]; then
        error_exit "$last_error within $elapsed_time seconds."
    fi

    echo "Retrying in $POLL_INTERVAL seconds..."
    sleep $POLL_INTERVAL
    elapsed_time=$((elapsed_time + POLL_INTERVAL))

done

echo "All allocs reattached correctly for node at $CLIENT_IP"
