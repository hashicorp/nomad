#!/usr/bin/env bash

set -xeuo pipefail


# Quality: nomad_agent_info: A GET call to /v1/agent/members returns the correct number of running servers and they are all aliv

RUNNING_SERVERS=$(nomad server members -json)
SERVERS_LENGTH=$(echo "$RUNNING_SERVERS" | jq 'length' )

if [ -z "$SERVERS_LENGTH" ];  then
    exit 100
fi

if [ "$SERVERS_LENGTH" -ne "$SERVERS" ]; then
    exit 101
fi

echo "$RUNNING_SERVERS" | jq -c '.[]' | while read -r node; do
  STATUS=$(echo "$node" | jq -r '.Status')

  if [ "$STATUS" != "alive" ]; then
     exit 102
  fi
done

RESULT=$(echo "$RUNNING_SERVERS" | jq -r "map(.last_log_index ) | unique | length == 1")
if [ "$RESULT" ne "true" ]; then
       exit 103
fi

echo "All SERVERS are alive and up to date."

# Quality: "nomad_CLIENTS_status: A GET call to /v1/CLIENTS returns the correct number of clients and they are all eligible and ready"
RUNNING_CLIENTS=$(nomad node status -json)
CLIENTS_LENGTH=$(echo "$RUNNING_CLIENTS" | jq 'length' )

if [ -z "$CLIENTS_LENGTH" ];  then
    exit 100
fi

if [ "$CLIENTS_LENGTH" -ne "$CLIENTS" ]; then
    exit 101
fi

echo "$RUNNING_CLIENTS" | jq -c '.[]' | while read -r node; do
  STATUS=$(echo "$node" | jq -r '.Status')

  if [ "$STATUS" != "ready" ]; then
     exit 102
  fi

  ELIGIBILITY=$(echo "$node" | jq -r '.SchedulingEligibility')

  if [ "$ELIGIBILITY" != "eligible" ]; then
     exit 103
  fi
done

echo "All CLIENTS are eligible and running."

# Quality: nomad_job_status: A GET call to /v1/jobs returns the correct number of jobs and they are all running.
