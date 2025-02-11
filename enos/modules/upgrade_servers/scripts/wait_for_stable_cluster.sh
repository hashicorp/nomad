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

while true; do  
    servers=$(nomad operator api /v1/operator/raft/configuration)
    leader=$(echo $servers | jq -r '[.Servers[] | select(.Leader == true)'])
    echo $servers | jq '.'
    echo $leader
    if [ $(echo "$leader" | jq 'length') -eq 1 ]; then
      break
    fi

    if [ "$elapsed_time" -ge "$MAX_WAIT_TIME" ]; then
        error_exit "No leader found after $elapsed_time seconds."
    fi

    echo "No leader found yet after $elapsed_time seconds. Retrying in $POLL_INTERVAL seconds..."
    sleep "$POLL_INTERVAL"
    elapsed_time=$((elapsed_time + POLL_INTERVAL))
done

last_config_index=$(echo $servers | jq -r '.Index')
echo "last_config_index: $last_config_index"

for ip in $SERVERS; do
while true; do  
        echo $ip
        node_info=$(nomad agent-info -address "https://$ip:4646" -json)
        if [ $? -ne 0 ]; then
            error_exit "Unable to get info for node at $ip"
        fi

        last_log_index=$(echo "$node_info" | jq -r '.stats.raft.last_log_index')
        if [ "$last_log_index" -ge "$last_config_index" ]; then
            break
        fi

        if [ "$elapsed_time" -ge "$MAX_WAIT_TIME" ]; then
            error_exit "Expected node at $ip to have last log index at least $last_config_index but found $last_log_index after $elapsed_time seconds."
        fi

        echo "Expected log at $leader_last_index, found $last_log_index. Retrying in $POLL_INTERVAL seconds..."
        sleep "$POLL_INTERVAL"
        elapsed_time=$((elapsed_time + POLL_INTERVAL))
    done    
done

echo "All servers are alive and up to date."
