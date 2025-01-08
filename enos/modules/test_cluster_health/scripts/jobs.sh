#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

# Quality: nomad_job_status: A GET call to /v1/jobs returns the correct number of jobs and they are all running.

RUNNING_JOBS=$(nomad job status)
JOBS_LENGTH=$(echo "$RUNNING_JOBS" | awk 'NR > 1 {count++} END {print count}')

if [ -z "$JOBS_LENGTH" ];  then
    echo "Error: No jobs found" 
    exit 1
fi

if [ "$JOBS_LENGTH" -ne "$JOBS" ]; then
    echo "Error: The number of jobs does not match the expected count"
    exit 1
fi

if [ -n "$(echo "$RUNNING_JOBS" | awk '{if ($2 != "running") print $1}')" ]; then
    echo "Error: Job not running"
    exit 1
fi

echo "All JOBS are running."

#if [ $(echo "$RUNNING_JOBS" | jq '[.[] | .Allocations | length] | add') nq "$ALLOCS"]; then
#  exit 1
#fi

#if [jq '[.[] | .Allocations | all(.State == "running")] | all' input.json
#]

# Quality: nomad_allocs_status: A GET call to /v1/allocs returns the correct number of allocations and they are all running.

echo "All allocs are running."