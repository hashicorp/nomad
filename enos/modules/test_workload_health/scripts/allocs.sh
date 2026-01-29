#!/usr/bin/env bash
# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

# Quality: nomad_allocs_status: A GET call to /v1/allocs returns the correct number of allocations and they are all running

# Quality: nomad_reschedule_alloc: A POST / PUT call to /v1/allocation/:alloc_id/stop results in the stopped allocation being rescheduled

error_exit() {
    printf 'Error: %s\n' "${1}"
    ALL_ALLOCS=$(nomad alloc status -json)
    mkdir -p /tmp/artifacts
    OUT="/tmp/artifacts/allocs.json"
    echo "$ALL_ALLOCS" > "$OUT"

    cat "$OUT" | jq -r '
        ["ID", "Node", "ClientStatus", "DesiredStatus", "JobID"],
        ["--------", "--------", "------------", "-------------", "---------------"],
        (.[] | [.ID[:8], .NodeID[:8], .ClientStatus, .DesiredStatus, .JobID])
        | @tsv' | column -ts $'\t'

    echo "full allocation status for debugging written to: $OUT"
    exit 1
}

MAX_WAIT_TIME=120
POLL_INTERVAL=2
elapsed_time=0

# convert the csv lists of jobs into an array for each type
IFS=',' read -r -a SERVICE_JOBS <<< "$SERVICE_JOBS"
IFS=',' read -r -a SYSTEM_JOBS <<< "$SYSTEM_JOBS"
IFS=',' read -r -a BATCH_JOBS <<< "$BATCH_JOBS"
IFS=',' read -r -a SYSBATCH_JOBS <<< "$SYSBATCH_JOBS"

# we'll collect a list of jobs missing allocs
declare -A MISSING_ALLOCS

last_error=

# checks that each service job has the expected count of running allocs
checkServiceJobs() {
    local job
    local expect
    local running
    local ok
    ok=0

    for job in "${SERVICE_JOBS[@]}"; do
        expect=$(nomad job inspect "$job" | jq '[.Job.TaskGroups[].Count] | add')
        running=$(nomad job status -json "$job" |
                      jq '.[].Allocations[] | select(.ClientStatus=="running").ID' |
                      wc -l)
        if [[ "$expect" != "$running" ]]; then
            MISSING_ALLOCS["$job"]=1
            last_error="Some jobs were missing expected running allocs: ${!MISSING_ALLOCS[*]}"
            ok=1
        fi
    done

    return "$ok"
}

# checks that each node has an alloc for each system job
checkSystemJobs() {
    local job
    local expect
    local running
    local ok
    ok=0

    for job in "${SYSTEM_JOBS[@]}"; do
        # every test system workload should run on every node
        running=$(nomad job status -json "$job" |
                      jq '.[].Allocations[] | select(.ClientStatus=="running").ID' |
                      wc -l)
        if [[ "$CLIENT_COUNT" != "$running" ]]; then
            MISSING_ALLOCS["$job"]=1
            last_error="Some jobs were missing expected running allocs: ${!MISSING_ALLOCS[*]}"
            ok=1
        fi
    done

    return "$ok"
}

checkBatchJobs() {
    local job
    local expect
    local running
    local reduce
    local ok
    ok=0

    for job in "${BATCH_JOBS[@]}"; do
        expect=$(nomad job inspect "$job" | jq '[.Job.TaskGroups[].Count] | add')
        running=$(nomad job status -json "$job" |
                      jq '.[].Allocations[] | select(.ClientStatus=="running").ID' |
                      wc -l)
        if [[ "$expect" == "$running" ]]; then
            continue
        fi
        # one or more allocs may have been on a drained node
        drained=$(nomad node status -json | jq -r '[.[] | select(.LastDrain != null).ID]')

        # get the count of complete allocations for this job that were on any of
        # the drained nodes; we can deduct these from the expected set
        added=$(nomad job status -json "$job" |
                     jq --argjson drained "$drained" \
                        '[ .[].Allocations[]
                           | select(.ClientStatus=="complete")
                           | select(.NodeID as $nodeID | any($drained[]; . == $nodeID)).ID
                         ] | length')
        running=$((running + added))
        if [ "$running" -lt "$expect" ]; then
            MISSING_ALLOCS["$job"]=1
            last_error="Some jobs were missing expected running allocs: ${!MISSING_ALLOCS[*]}"
            ok=1
        fi
    done

    return "$ok"
}

checkSysbatchJobs() {
    local job
    local expect
    local running
    local reduce
    local ok
    ok=0

    for job in "${SYSBATCH_JOBS[@]}"; do
        # every test sysbatch workload should run on every node
        expect="$CLIENT_COUNT"
        running=$(nomad job status -json "$job" |
                      jq '.[].Allocations[] | select(.ClientStatus=="running").ID' |
                      wc -l)
        if [[ "$expect" == "$running" ]]; then
            continue
        fi

        # one or more allocs may have been on a drained node
        drained=$(nomad node status -json | jq -r '[.[] | select(.LastDrain != null).ID]')

        # get the count of complete allocations for this job that were on any of
        # the drained nodes; we can deduct these from the expected set
        added=$(nomad job status -json "$job" |
                     jq --argjson drained "$drained" \
                        '[ .[].Allocations[]
                           | select(.ClientStatus=="complete")
                           | select(.NodeID as $nodeID | any($drained[]; . == $nodeID)).ID
                         ] | length')
        running=$((running + added))
        if [ "$running" -lt "$expect" ]; then
            MISSING_ALLOCS["$job"]=1
            last_error="Some jobs were missing expected running allocs: ${!MISSING_ALLOCS[*]}"
            ok=1
        fi
    done

    return "$ok"
}

checkGroup() {
    unset "MISSING_ALLOCS[@]"
    elapsed_time=0
    fn=${1}
    while true; do
        $fn && break
        if [ "$elapsed_time" -ge "$MAX_WAIT_TIME" ]; then
            error_exit "$last_error"
        fi

        echo "Not all allocs were running. Retrying in $POLL_INTERVAL seconds..."
        sleep $POLL_INTERVAL
        elapsed_time=$((elapsed_time + POLL_INTERVAL))
    done
}

# stop a random service job's allocation
stopAllocAndWait() {
    local job
    local allocID
    local random_index

    random_index=$((RANDOM % ${#SERVICE_JOBS[@]}))
    job=${SERVICE_JOBS[$random_index]}
    allocID=$(nomad job status -json "$job" |
                  jq -r '.[].Allocations[0] | select(.ClientStatus=="running").ID')

    nomad alloc stop "$allocID" || error_exit "Failed to stop allocation $allocID"
}

echo "Waiting for all expected allocs to be running."
checkGroup checkServiceJobs
checkGroup checkSystemJobs
checkGroup checkBatchJobs
checkGroup checkSysbatchJobs

echo "Stopping a random service job allocation."
stopAllocAndWait

echo "Waiting for all expected allocs to be running after reschedule."
checkGroup checkServiceJobs

echo "All expected allocs running."
