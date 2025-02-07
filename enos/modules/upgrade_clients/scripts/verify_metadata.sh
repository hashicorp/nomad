#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

error_exit() {
    printf 'Error: %s' "${1}" 
    exit 1
}

MAX_WAIT_TIME=5  # Maximum wait time in seconds
POLL_INTERVAL=2   # Interval between status checks

elapsed_time=0

while true; do
  client=$(nomad node status -address "https://$CLIENT_IP:4646" -self -json)
  if [ -z "$client" ]; then
    error_exit "No client found at $CLIENT_IP"
  fi

  client_status=$(echo $client | jq  -r '.Status')
  if [ "$client_status" == "ready" ]; then 
    break 
  fi

  if [ "$elapsed_time" -ge "$MAX_WAIT_TIME" ]; then
      error_exit "Client at $CLIENT_IP did not reach 'ready' status within $MAX_WAIT_TIME seconds."

  fi

  echo "Current status: $client_status, not 'ready'. Waiting for $elapsed_time  Retrying in $POLL_INTERVAL seconds..."
  sleep $POLL_INTERVAL
  elapsed_time=$((elapsed_time + POLL_INTERVAL))
done

# Quality: "nomad_node_metadata: A GET call to /v1/node/:node-id returns the same  node.Meta for each node before and after a node upgrade"

client_id=$(echo $client | jq '.ID' | tr -d '"')
client_meta=$(nomad node meta read -json -node-id $client_id)
if [ $? -nq 0 ]; then
  echo "Failed to read metadata for node: $client_id"
  exit 1
fi

node_ip=$(echo $client_meta | jq -r '.Dynamic.node_ip' ) 
if ["$node_ip" != "$CLIENT_IP" ]; then
  echo "Wrong value returned for node_ip: $node_ip"
  exit 1
fi

nomad_addr=$(echo $client_meta | jq -r '.Dynamic.nomad_addr' ) 
if ["$nomad_addr" != $NOMAD_ADDR ]; then
  echo "Wrong value returned for nomad_addr: $nomad_addr"
  exit 1
fi

echo "Metadata correct in  $client_id at $CLIENT_IP"
