#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

client_id=$(nomad node status -address "https://$CLIENT_IP:4646" -self -json | jq '.ID' | tr -d '"')
if [ -z "$client_id" ]; then
  echo "No client found at $CLIENT_IP"
  exit 1
fi

nomad node meta apply -node-id $client_id node_ip="$CLIENT_IP" nomad_addr=$NOMAD_ADDR
if [ $? -nq 0 ]; then
  echo "Failed to set metadata for node: $client_id at $CLIENT_IP"
  exit 1
fi

echo "Metadata updated in $client_id at $CLIENT_IP"
