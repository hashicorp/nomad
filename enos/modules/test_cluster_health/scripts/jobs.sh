#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

error_exit() {
    printf 'Error: %s' "${1}"
    exit 1
}

# Quality: nomad_job_status: A GET call to /v1/jobs returns the correct number of jobs and they are all running.

jobs_length=$(nomad job status| awk '$4 == "running" {count++} END {print count+0}')

if [ -z "$jobs_length" ];  then
    error_exit "No jobs found"
fi

if [ "$jobs_length" -ne "$JOB_COUNT" ]; then
    error_exit "The number  of running jobs ($jobs_length) does not match the expected count ($JOB_COUNT) $(nomad job status | awk 'NR > 1 && $4 != "running" {print $4}') "
fi

echo "All JOBS are running."
