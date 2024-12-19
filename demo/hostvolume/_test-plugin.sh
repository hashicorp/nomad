#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

help() {
  cat <<EOF
Runs a Dynamic Host Volume plugin with dummy values.

Usage:
  $0 <plugin> <operation> [target dir] [uuid]

Args:
  plugin: path to plugin executable
  operation: fingerprint, create, or delete
    create and delete must be idempotent.
    any other operation will be passed into the plugin,
    to see how it handles invalid operations.
  target dir: directory to create the volume (defaults to /tmp)
  uuid: volume id to use (usually assigned by Nomad;
    defaults to 74564d17-ce50-0bc1-48e5-6feaa41ede48)

Examples:
  $0 ./example-plugin-mkfs fingerprint
  $0 ./example-plugin-mkfs create
  $0 ./example-plugin-mkfs create /some/other/place
  $0 ./example-plugin-mkfs delete
EOF
}

if [[ $# -eq 0 || "$*" =~ -h ]]; then
  help
  exit
fi
if [ $# -lt 2 ]; then
  help
  exit 1
fi

plugin="$1"
op="$2"
alloc_mounts="${3:-/tmp}"
uuid="${4:-74564d17-ce50-0bc1-48e5-6feaa41ede48}"

case $op in
  fingerprint)
    args='fingerprint'
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
eval "$plugin $args"
