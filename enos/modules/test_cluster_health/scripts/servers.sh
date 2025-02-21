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
last_error=
leader_last_index=
leader_last_term=

# Quality: nomad_agent_info: A GET call to /v1/agent/members returns the correct number of running servers and they are all alive

checkAutopilotHealth() {
    local autopilotHealth servers_healthy leader
    autopilotHealth=$(nomad operator autopilot health -json) || {
        last_error="Could not read autopilot health"
        return 1
    }
    servers_healthy=$(echo "$autopilotHealth" |
                          jq -r '[.Servers[] | select(.Healthy == true) | .ID] | length')

    if [ "$servers_healthy" -eq 0 ]; then
        error_exit "No servers found."
    fi

    if [ "$servers_healthy" -eq "$SERVER_COUNT" ]; then
        leader=$(echo "$autopilotHealth" | jq -r '.Servers[] | select(.Leader == true)')
        leader_last_index=$(echo "$leader" | jq -r '.LastIndex')
        leader_last_term=$(echo "$leader" | jq -r '.LastTerm')
        return 0
    fi

    last_error="Expected $SERVER_COUNT healthy servers but have $servers_healthy"
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

# Quality: nomad_agent_info_self: A GET call to /v1/agent/self against every server returns the same last_log_index as the leader"
# We use the leader's last log index to use as teh measure for the other servers.

checkServerHealth() {
    local ip node_info
    ip=$1
    echo "Checking server health for $ip"

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

echo "All servers are alive and up to date."
