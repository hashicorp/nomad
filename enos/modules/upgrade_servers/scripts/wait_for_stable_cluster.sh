#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

error_exit() {
    printf 'Error: %s' "${1}"
    exit 1
}

MAX_WAIT_TIME=10 #40
POLL_INTERVAL=2

elapsed_time=0
last_config_index=
last_error=

checkRaftConfiguration() {
    local raftConfig leader
    raftConfig=$(nomad operator api /v1/operator/raft/configuration) || return 1
    leader=$(echo "$raftConfig" | jq -r '[.Servers[] | select(.Leader == true)'])

    echo "$raftConfig" | jq '.'
    echo "$leader"
    if [ "$(echo "$leader" | jq 'length')" -eq 1 ]; then
        last_config_index=$(echo "$raftConfig" | jq -r '.Index')
        echo "last_config_index: $last_config_index"
        return 0
    fi

    last_error="No leader found"
    return 1
}

while true; do
    checkRaftConfiguration && break
    if [ "$elapsed_time" -ge "$MAX_WAIT_TIME" ]; then
        error_exit "${last_error} after $elapsed_time seconds."
    fi

    echo "${last_error} after $elapsed_time seconds. Retrying in $POLL_INTERVAL seconds..."
    sleep "$POLL_INTERVAL"
    elapsed_time=$((elapsed_time + POLL_INTERVAL))
done


# reset timer
elapsed_time=0
last_log_index=

checkServerHealth() {
    local ip node_info
    ip=$1
    echo "Checking server health for $ip"

    node_info=$(nomad agent-info -address "https://$ip:4646" -json) \
        || error_exit "Unable to get info for node at $ip"

    last_log_index=$(echo "$node_info" | jq -r '.stats.raft.last_log_index')
    if [ "$last_log_index" -ge "$last_config_index" ]; then
        return 0
    fi

    last_error="Expected node at $ip to have last log index at least $last_config_index but found $last_log_index"
    return 1
}

for ip in $SERVERS; do
    while true; do
        checkServerHealth "$ip" && break

        if [ "$elapsed_time" -ge "$MAX_WAIT_TIME" ]; then
            error_exit "$last_error after $elapsed_time seconds."
        fi

        echo "${last_error} after $elapsed_time seconds. Retrying in $POLL_INTERVAL seconds..."
        sleep "$POLL_INTERVAL"
        elapsed_time=$((elapsed_time + POLL_INTERVAL))
    done
done

echo "All servers are alive and up to date."
