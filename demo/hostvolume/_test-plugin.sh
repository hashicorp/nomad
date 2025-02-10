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
volumes_dir="${3:-/tmp}"
uuid="${4:-74564d17-ce50-0bc1-48e5-6feaa41ede48}"
node_id='0b62d807-6101-a80f-374d-e1c430abbf47'
plugin_dir="$(dirname "$plugin")" 

case $op in
  fingerprint)
    args='fingerprint'
    export DHV_OPERATION='fingerprint'
    ;;

	create)
    args='create'
    export DHV_OPERATION='create'
    export DHV_VOLUMES_DIR="$volumes_dir"
    export DHV_VOLUME_NAME=test
    export DHV_VOLUME_ID="$uuid"
    export DHV_NODE_ID="$node_id"
    export DHV_CAPACITY_MAX_BYTES=50000000 # 50mb
    export DHV_CAPACITY_MIN_BYTES=50000000 # 50mb
    export DHV_PARAMETERS='{"a": "ayy"}'
    export DHV_PLUGIN_DIR="$plugin_dir"
    ;;

  delete)
    args='delete'
    export DHV_OPERATION='delete'
    export DHV_VOLUMES_DIR="$volumes_dir"
    export DHV_NODE_ID="$node_id"
    export DHV_VOLUME_NAME=test
    export DHV_VOLUME_ID="$uuid"
    export DHV_PARAMETERS='{"a": "ayy"}'
    export DHV_PLUGIN_DIR="$plugin_dir"
    ;;

  *)
    args="$*"
	  ;;
esac

set -x
eval "$plugin $args"
