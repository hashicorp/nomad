#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

if [[ $# -eq 0 || "$*" =~ -h ]]; then
  cat <<EOF
Runs a Dynamic Host Volume plugin with dummy values.

Usage:
  $(basename "$0") <operation>

Operations:
  create, delete, version
  any other operation will be passed to the plugin

Environment variables:
  PLUGIN: executable to run (default ./example-host-volume)
  TARGET_DIR: path to place the mount dir (default /tmp,
    usually {nomad data dir}/alloc_mounts)
EOF
  exit
fi

op="$1"
shift

plugin="${PLUGIN:-./example-host-volume}"
alloc_mounts="${TARGET_DIR:-/tmp}"
uuid='74564d17-ce50-0bc1-48e5-6feaa41ede48'

case $op in
  version)
    args='version'
    stdin=''
    ;;

	create)
    args="create $alloc_mounts/$uuid"
    stdin='{"ID":"'$uuid'","Name":"test","PluginID":"example-host-volume","NodeID":"0b62d807-6101-a80f-374d-e1c430abbf47","RequestedCapacityMinBytes":50000000,"RequestedCapacityMaxBytes":50000000,"Parameters":null}'
    export HOST_PATH="$alloc_mounts/$uuid"
    export VOLUME_NAME=test
    export NODE_ID=0b62d807-6101-a80f-374d-e1c430abbf47
    export CAPACITY_MAX_BYTES=50000000 # 50mb
    export CAPACITY_MIN_BYTES=50000000 # 50mb
    # TODO(db): check stdout
    ;;

  delete)
    args="delete $alloc_mounts/$uuid"
    stdin='{"ID":"'$uuid'","PluginID":"example-host-volume","NodeID":"0b62d807-6101-a80f-374d-e1c430abbf47","HostPath":"'"$alloc_mounts/$uuid"'",","Parameters":null}'
    export HOST_PATH="$alloc_mounts/$uuid"
    ;;

  *)
    args="$*"
    stdin=''
	  ;;
esac

export OPERATION="$op"
set -x
echo "$stdin" | eval "$plugin $* $args"
