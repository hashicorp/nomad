#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

# Quality: "nomad_CLIENTS_status: A GET call to /v1/CLIENTS returns the correct number of clients and they are all eligible and ready"
RUNNING_CLIENTS=$(nomad node status -json)
CLIENTS_LENGTH=$(echo "$RUNNING_CLIENTS" | jq 'length' )

if [ -z "$CLIENTS_LENGTH" ];  then
    echo "Error: No clients found" 
    exit 1
fi

if [ "$CLIENTS_LENGTH" -ne "$CLIENTS" ]; then
    echo "Error: The number of clients does not match the expected count"
    exit 1
fi

echo "$RUNNING_CLIENTS" | jq -c '.[]' | while read -r node; do
  STATUS=$(echo "$node" | jq -r '.Status')

  if [ "$STATUS" != "ready" ]; then
    echo "Error: Client not alive"
    exit 1
  fi

  ELIGIBILITY=$(echo "$node" | jq -r '.SchedulingEligibility')

  if [ "$ELIGIBILITY" != "eligible" ]; then
    echo "Error: Client not eligible"
    exit 1
  fi
done

echo "All CLIENTS are eligible and running."