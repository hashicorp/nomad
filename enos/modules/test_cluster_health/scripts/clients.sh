#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

error_exit() {
    echo "Error: $1"
    exit 1
}

# Quality: "nomad_CLIENTS_status: A GET call to /v1/CLIENTS returns the correct number of clients and they are all eligible and ready"
clients=$(nomad node status -json)
running_clients=$(echo $clients | jq '[.[] | select(.Status == "ready")]')
clients_length=$(echo "$running_clients" | jq 'length' )

if [ -z "$clients_length" ];  then
    error_exit "No clients found"
fi

if [ "$clients_length" -ne "$CLIENTS" ]; then
     error_exit "Unexpected number of clients are ready $(echo $clients | jq '.[] | select(.Status != "ready") | .Name')"

fi

echo "$running_clients" | jq -c '.[]' | while read -r node; do
  status=$(echo "$node" | jq -r '.Status')

  eligibility=$(echo "$node" | jq -r '.SchedulingEligibility')

  if [ "$eligibility" != "eligible" ]; then
    error_exit "Client not eligible $(echo "$node" | jq -r '.Name')"
  fi
done

echo "All CLIENTS are eligible and running."
