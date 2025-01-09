#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

error_exit() {
    echo "Error: $1"
    exit 1
}

# Quality: nomad_agent_info: A GET call to /v1/agent/members returns the correct number of running servers and they are all aliv
servers=$(nomad server members -json )
running_servers=$(echo $servers | jq '[.[] | select(.Status == "alive")]')
servers_length=$(echo "$running_servers" | jq 'length' )

if [ -z "$servers_length" ];  then
    error_exit "No servers found"
fi

if [ "$servers_length" -ne "$SERVERS" ]; then
    error_exit "Unexpected number of servers are alive $(echo $servers | jq '.[] | select(.Status != "alive") | .Name')"
fi

if [ $(echo "$running_servers" | jq -r "map(.last_log_index ) | unique | length == 1") != "true" ]; then
    error_exit "Servers not up to date"
fi

echo "All SERVERS are alive and up to date."
