#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

error_exit() {
    printf 'Error: %s' "${1}" 
    exit 1
}

# Quality: nomad_agent_info: A GET call to /v1/agent/members returns the correct number of running servers and they are all alive

servers=$(nomad server members -json )
running_servers=$(echo $servers | jq '[.[] | select(.Status == "alive")]')
servers_length=$(echo "$running_servers" | jq 'length' )

if [ -z "$servers_length" ];  then
    error_exit "No servers found"
fi

if [ "$servers_length" -ne "$SERVER_COUNT" ]; then
    error_exit "Unexpected number of servers are alive: $servers_length\n$(echo $servers | jq '.[] | select(.Status != "alive") | .Name')"
fi

# Quality: nomad_agent_info_self: A GET call to /v1/agent/self against every server returns the same last_log_index for all of them"

last_index=""

INDEX_WINDOW=5 # All the servers should be within +5/-5 raft log indexes from one another.

for ip in $SERVERS; do
    
    last_log_index=$(nomad agent-info -address "https://$ip:4646" -json | jq -r '.stats.raft.last_log_index')
    if [ -n "$last_index" ]; then
        if (( last_log_index < last_index - INDEX_WINDOW || last_log_index > last_index + INDEX_WINDOW )); then
            error_exit "Servers not on the same index! $ip is at index: $last_log_index, previous index: $last_index"
        fi
    fi

    last_index="$last_log_index"
done

echo "All SERVERS are alive and up to date."
