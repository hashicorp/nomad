#!/usr/bin/env bash
# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

# Quality: nomad_job_status: A GET call to /v1/jobs returns the correct number
# of jobs and they are all running.

error_exit() {
    printf 'Error: %s' "${1}"
    nomad job status
    exit 1
}

# jobs should move from "pending" to "running" fairly quickly
MAX_WAIT_TIME=30
POLL_INTERVAL=2
elapsed_time=0
last_error=

# convert the csv list of jobs into an array
IFS=',' read -r -a JOBS <<< "$JOBS"
declare -A NON_RUNNING_JOBS

checkRunningJobs() {
    unset "NON_RUNNING_JOBS[@]"
    local status
    local job
    local ok
    ok=0
    for job in "${JOBS[@]}"; do
        status=$(nomad job inspect "$job" | jq '.Job.Status')
        if [[ "$status" != "running" ]]; then
            NON_RUNNING_JOBS["$job"]=1
        fi
    done

    if [[ ${#NON_RUNNING_JOBS[@]} != 0 ]]; then
        last_error="Some expected jobs were not running: ${!NON_RUNNING_JOBS[*]}"
        ok=1
    fi

    return "$ok"
}

while true; do
    checkRunningJobs && break
    if [ "$elapsed_time" -ge "$MAX_WAIT_TIME" ]; then
        error_exit "$last_error within $elapsed_time seconds."
    fi

    echo "Not all jobs were running. Retrying in $POLL_INTERVAL seconds..."
    sleep $POLL_INTERVAL
    elapsed_time=$((elapsed_time + POLL_INTERVAL))
done

echo "All expected jobs are running."
