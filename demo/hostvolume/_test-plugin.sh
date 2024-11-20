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
    ;;

	create)
    args="create $alloc_mounts/$uuid"
    export HOST_PATH="$alloc_mounts/$uuid"
    export VOLUME_NAME=test
    export NODE_ID=0b62d807-6101-a80f-374d-e1c430abbf47
    export CAPACITY_MAX_BYTES=50000000 # 50mb
    export CAPACITY_MIN_BYTES=50000000 # 50mb
    export PARAMETERS='{"a": "ayy"}'
    # db TODO(1.10.0): check stdout
    ;;

  delete)
    args="delete $alloc_mounts/$uuid"
    export HOST_PATH="$alloc_mounts/$uuid"
    export PARAMETERS='{"a": "ayy"}'
    ;;

  *)
    args="$*"
	  ;;
esac

export OPERATION="$op"
set -x
eval "$plugin $* $args"
