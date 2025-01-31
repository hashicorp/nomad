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

for ip in $SERVERS; do
    
    last_log_index=$(nomad agent-info -address "https://$ip:4646" -json | jq -r '.stats.raft.last_log_index')
    if [ -n "$last_index" ] && [ "$last_log_index" -ne "$last_index" ]; then
        error_exit "Servers not on the same index. $ip on  index: $last_index,  previous read index: $last_log_index"
    fi

    last_index="$last_log_index"
done

echo "All SERVERS are alive and up to date."
