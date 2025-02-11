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
    servers=$(nomad operator autopilot health -json) || { echo "Failed to fetch Nomad health status, "; }
    leader=$(echo "$servers" | jq -r '[.Servers[] | select(.Leader == true) | .ID] | length')

    if [ "$leader" -eq 1 ]; then
      break
    fi

    if [ "$elapsed_time" -ge "$MAX_WAIT_TIME" ]; then
        error_exit "No leader found after $elapsed_time seconds."
    fi

    echo "No leader found yet after $elapsed_time seconds. Retrying in $POLL_INTERVAL seconds..."
    sleep "$POLL_INTERVAL"
    elapsed_time=$((elapsed_time + POLL_INTERVAL))
done

leader=$(echo $servers | jq -r '.Servers[] | select(.Leader == true)')
leader_last_index=$(echo $leader | jq -r '.LastIndex')
leader_last_term=$(echo $leader | jq -r '.LastTerm')
echo "working with $leader_last_index $leader_last_term"

for ip in $SERVERS; do
while true; do  
        echo $ip
        node_info=$(nomad agent-info -address "https://$ip:4646" -json)
        if [ $? -ne 0 ]; then
            error_exit "Unable to get info for node at $ip"
        fi

        last_log_index=$(echo "$node_info" | jq -r '.stats.raft.last_log_index')
        last_leader_term=$(echo "$node_info" | jq -r '.stats.raft.last_log_term')
        echo "reading $last_log_index $last_leader_term"
        if [ "$last_log_index" -ge "$leader_last_index" ] && [ "$last_leader_term" -ge "$leader_last_term" ]; then
            break
        fi

        if [ "$elapsed_time" -ge "$MAX_WAIT_TIME" ]; then
            error_exit "Expected node at $ip to have last log index $leader_last_index and last term $leader_last_term, but found $last_log_index and $last_leader_term after $elapsed_time seconds."
        fi

        echo "Expected log at $leader_last_index, found $last_log_index. Retrying in $POLL_INTERVAL seconds..."
        sleep "$POLL_INTERVAL"
        elapsed_time=$((elapsed_time + POLL_INTERVAL))
    done    
done

echo "All servers are alive and up to date."
