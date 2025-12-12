#!/usr/bin/env bash
# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

error_exit() {
    printf 'Error: %s' "${1}"
    nomad job status
    exit 1
}

# Quality: nomad_job_status: A GET call to /v1/jobs returns the correct number
# of jobs and they are all running.

# jobs should move from "pending" to "running" fairly quickly
MAX_WAIT_TIME=30
POLL_INTERVAL=2
elapsed_time=0
last_error=

checkRunningJobsCount() {
    jobs_length=$(nomad job status| awk '$4 == "running" {count++} END {print count+0}') || {
        last_error="Could not query job status"
        return 1
    }

    if [ -z "$jobs_length" ];  then
        last_error="No running jobs found"
        return 1
    fi

    if [ "$jobs_length" -ne "$JOB_COUNT" ]; then
        last_error="The number of running jobs ($jobs_length) does not match the expected count ($JOB_COUNT)"
        return 1
    fi
}


while true; do
    # reset
    jobs_length=

    checkRunningJobsCount && break
    if [ "$elapsed_time" -ge "$MAX_WAIT_TIME" ]; then
        error_exit "$last_error within $elapsed_time seconds."
    fi

    echo "Expected $JOB_COUNT running jobs, found ${jobs_length}. Retrying in $POLL_INTERVAL seconds..."
    sleep $POLL_INTERVAL
    elapsed_time=$((elapsed_time + POLL_INTERVAL))
done

echo "Expected number of jobs ($JOB_COUNT) are running."
