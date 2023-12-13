#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


# Run the hostpath plugin and create some volumes, and then claim them.
set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
VOLUME_BASE_NAME=test-volume

run_plugin() {
    local expected
    expected=$(nomad node status | grep -cv ID)
    echo "$ nomad job run ./plugin.nomad"
    nomad job run "${DIR}/plugin.nomad"

    while :
    do
        nomad plugin status hostpath \
            | grep "Nodes Healthy        = $expected" && break
        sleep 2
    done
    echo
    echo "$ nomad plugin status hostpath"
    nomad plugin status hostpath
}

create_volumes() {
    echo
    echo "$ cat hostpath.hcl | sed | nomad volume create -"
    sed -e "s/VOLUME_NAME/${VOLUME_BASE_NAME}[0]/" \
        "${DIR}/hostpath.hcl" | nomad volume create -

    echo
    echo "$ cat hostpath.hcl | sed | nomad volume create -"
    sed -e "s/VOLUME_NAME/${VOLUME_BASE_NAME}[1]/" \
        "${DIR}/hostpath.hcl" | nomad volume create -
}

claim_volumes() {
    echo
    echo "$ nomad job run ./redis.nomad"
    nomad job run "${DIR}/redis.nomad"
}

show_status() {
    echo
    echo "$ nomad volume status"
    nomad volume status
}

run_plugin
create_volumes
claim_volumes
show_status
