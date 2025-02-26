#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

error_exit() {
    printf 'Error: %s' "${1}"
    exit 1
}

MAX_WAIT_TIME=60
POLL_INTERVAL=2

elapsed_time=0
last_error=
leader_last_index=
leader_last_term=

checkAutopilotHealth() {
    local autopilotHealth leader
    autopilotHealth=$(nomad operator autopilot health -json) || {
        last_error="Could not read autopilot health"
        return 1
    }
    leader=$(echo "$autopilotHealth" | jq -r '[.Servers[] | select(.Leader == true)]')
    if [ "$(echo "$leader" | jq 'length')" -eq 1 ]; then
        leader_last_index=$(echo "$leader" | jq -r '.[0].LastIndex')
        leader_last_term=$(echo "$leader" | jq -r '.[0].LastTerm')
        return 0
    fi

    last_error="No leader found"
    return 1
}

while true; do
    checkAutopilotHealth && break

    if [ "$elapsed_time" -ge "$MAX_WAIT_TIME" ]; then
        error_exit "$last_error after $elapsed_time seconds."
    fi

    echo "$last_error after $elapsed_time seconds. Retrying in $POLL_INTERVAL seconds..."
    sleep "$POLL_INTERVAL"
    elapsed_time=$((elapsed_time + POLL_INTERVAL))
done

echo "Leader found"

checkServerHealth() {
    local ip node_info
    ip=$1
    echo "Checking server $ip is up to date"

    node_info=$(nomad agent-info -address "https://$ip:4646" -json) \
        || error_exit "Unable to get info for node at $ip"

    last_log_index=$(echo "$node_info" | jq -r '.stats.raft.last_log_index')
    last_log_term=$(echo "$node_info" | jq -r '.stats.raft.last_log_term')

    if [ "$last_log_index" -ge "$leader_last_index" ] &&
           [ "$last_log_term" -ge "$leader_last_term" ]; then
        return 0
    fi

    last_error="Expected node at $ip to have last log index $leader_last_index and last term $leader_last_term, but found $last_log_index and $last_log_term"
    return 1
}

for ip in $SERVERS; do
    while true; do
        checkServerHealth "$ip" && break

        if [ "$elapsed_time" -ge "$MAX_WAIT_TIME" ]; then
            error_exit "$last_error after $elapsed_time seconds."
        fi

        echo "$last_error after $elapsed_time seconds. Retrying in $POLL_INTERVAL seconds..."
        sleep "$POLL_INTERVAL"
        elapsed_time=$((elapsed_time + POLL_INTERVAL))
    done
done

echo "There is a leader and all servers are alive and up to date."
