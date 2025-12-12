#!/usr/bin/env bash
# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

# note: the stdout from this script gets read in as JSON to a later step, so
# it's critical we only emit other text if we're failing anyways
error_exit() {
    printf 'Error: %s' "${1}"
    exit 1
}

# we have a client IP and not a node ID, so query that node via 'node status
# -self' to get its ID
NODE_ID=$(nomad node status \
                -allocs -address="https://${CLIENT_IP}:4646" -self -json | jq -r '.ID')

# dump the allocs for this node only, keeping only client-relevant data and not
# the full jobspec. We only want the running allocations because we might have
# previously drained this node, which will mess up our expected counts.
nomad alloc status -json | \
    jq -r --arg NODE_ID "$NODE_ID" \
       '[ .[] | select(.NodeID == $NODE_ID and .ClientStatus == "running") | {ID: .ID, Name: .Name, ClientStatus: .ClientStatus, TaskStates: .TaskStates}]'
