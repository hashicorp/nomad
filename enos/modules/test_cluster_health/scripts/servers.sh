#!/usr/bin/env bash

set -euo pipefail

# Quality: nomad_agent_info: A GET call to /v1/agent/members returns the correct number of running servers and they are all aliv

RUNNING_SERVERS=$(nomad server members -json)
SERVERS_LENGTH=$(echo "$RUNNING_SERVERS" | jq 'length' )

if [ -z "$SERVERS_LENGTH" ];  then
    echo "Error: No servers found" 
    exit 1
fi

if [ "$SERVERS_LENGTH" -ne "$SERVERS" ]; then
    echo "Error: The number of servers does not match the expected count"
    exit 1
fi

echo "$RUNNING_SERVERS" | jq -c '.[]' | while read -r node; do
  STATUS=$(echo "$node" | jq -r '.Status')

  if [ "$STATUS" != "alive" ]; then
    echo "Error: Server not alive"
    exit 1
  fi
done

RESULT=$(echo "$RUNNING_SERVERS" | jq -r "map(.last_log_index ) | unique | length == 1")
if [ "$RESULT" != "true" ]; then
    echo "Error: Server not up to date"
    exit 1
fi

echo "All SERVERS are alive and up to date."