#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

error_exit() {
    printf 'Error: %s' "${1}" 
    exit 1
}

# Servers version
server_versions=$(nomad server members -json | jq -r '[.[] | select(.Status == "alive") | .Tags.build] | unique')

if [ "$(echo "$server_versions" | jq 'length')" -eq 0 ]; then
    error_exit "Unable to get servers version"
fi

if [ "$(echo "$server_versions" | jq 'length')" -ne 1 ]; then
    error_exit "Servers are running different versions: $(echo "$server_versions" | jq -c '.')"
fi

final_version=$(echo "$server_versions" | jq -r '.[0]'| xargs)
SERVERS_VERSION=$(echo "$SERVERS_VERSION" | xargs)

if [ "$final_version" != "$SERVERS_VERSION" ]; then
    error_exit "Servers are not running the correct version. Found: $final_version, Expected: $SERVERS_VERSION"
fi

echo "All servers are running Nomad version $SERVERS_VERSION"

# Clients version
clients_versions=$(nomad node status -json | jq -r '[.[] | select(.Status == "ready") | .Version] | unique')


if [ "$(echo "$clients_versions" | jq 'length')" -eq 0 ]; then
    error_exit "Unable to get clients version"
fi


if [ "$(echo "$clients_versions" | jq 'length')" -ne 1 ]; then
    error_exit "Clients are running different versions: $(echo "$clients_versions" | jq -c '.')"
fi

final_version=$(echo "$clients_versions" | jq -r '.[0]'| xargs)
CLIENTS_VERSION=$(echo "$CLIENTS_VERSION" | xargs)

if [ "$final_version" != "$CLIENTS_VERSION" ]; then
    error_exit "Clients are not running the correct version. Found: $final_version, Expected: $CLIENTS_VERSION"
fi

echo "All clients are running Nomad version $CLIENTS_VERSION"
