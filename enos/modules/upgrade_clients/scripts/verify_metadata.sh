#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

error_exit() {
    printf 'Error: %s' "${1}"
    exit 1
}

MAX_WAIT_TIME=10  # Maximum wait time in seconds
POLL_INTERVAL=2   # Interval between status checks

elapsed_time=0
last_error=
client_id=

checkClientReady() {
    local client client_status
    echo "Checking client health for $CLIENT_IP"

    client=$(nomad node status -address "https://$CLIENT_IP:4646" -self -json) || {
        last_error="Unable to get info for node at $CLIENT_IP"
        return 1
    }
    client_status=$(echo "$client" | jq  -r '.Status')
    if [ "$client_status" == "ready" ]; then
        client_id=$(echo "$client" | jq '.ID' | tr -d '"')
        last_error=
        return 0
    fi

    last_error="Node at $CLIENT_IP is ${client_status}, not ready"
    return 1
}

while true; do
    checkClientReady && break
    if [ "$elapsed_time" -ge "$MAX_WAIT_TIME" ]; then
        error_exit "$last_error within $elapsed_time seconds."
        exit 1
    fi

    echo "$last_error within $elapsed_time seconds. Retrying in $POLL_INTERVAL seconds..."
    sleep "$POLL_INTERVAL"
    elapsed_time=$((elapsed_time + POLL_INTERVAL))
done

# Quality: "nomad_node_metadata: A GET call to /v1/node/:node-id returns the same  node.Meta for each node before and after a node upgrade"

if ! client_meta=$(nomad node meta read -json -node-id "$client_id"); then
    echo "Failed to read metadata for node: $client_id"
    exit 1
fi

meta_node_ip=$(echo "$client_meta" | jq -r '.Dynamic.node_ip' )
if [ "$meta_node_ip" != "$CLIENT_IP" ]; then
  echo "Wrong value returned for node_ip: $meta_node_ip"
  exit 1
fi

meta_nomad_addr=$(echo "$client_meta" | jq -r '.Dynamic.nomad_addr' )
if [ "$meta_nomad_addr" != "$NOMAD_ADDR" ]; then
  echo "Wrong value returned for nomad_addr: $meta_nomad_addr"
  exit 1
fi

echo "Metadata correct in  $client_id at $CLIENT_IP"
