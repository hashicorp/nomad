#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# plugin for host_volume_plugin_test.go
set -xeuo pipefail

env 1>&2

test "$1" == "$DHV_OPERATION"

echo 'all operations should ignore stderr' 1>&2

# since we will be running `rm -rf` (frightening),
# check to make sure DHV_HOST_PATH has a uuid shape in it.
# Nomad generates a volume ID and includes it in the path.
validate_path() {
  if [[ ! "$DHV_HOST_PATH" =~ "test-vol-id" ]]; then
    1>&2 echo "expected test vol ID in the DHV_HOST_PATH; got: '$DHV_HOST_PATH'"
    exit 1
  fi
}

case $1 in
  fingerprint)
    echo '{"version": "0.0.2"}' ;;
  create)
    test "$2" == "$DHV_HOST_PATH"
    test "$DHV_NODE_ID" == 'test-node'
    test "$DHV_VOLUME_NAME" == 'test-vol-name'
    test "$DHV_VOLUME_ID" == 'test-vol-id'
    test "$DHV_PARAMETERS" == '{"key":"val"}'
    test "$DHV_CAPACITY_MIN_BYTES" -eq 5
    test "$DHV_CAPACITY_MAX_BYTES" -eq 10
    test "$DHV_PLUGIN_DIR" == './test_fixtures'
    validate_path "$DHV_HOST_PATH"
    mkdir "$2"
    printf '{"path": "%s", "bytes": 5, "context": %s}' "$2" "$DHV_PARAMETERS"
    ;;
  delete)
    test "$2" == "$DHV_HOST_PATH"
    test "$DHV_NODE_ID" == 'test-node'
    test "$DHV_VOLUME_NAME" == 'test-vol-name'
    test "$DHV_VOLUME_ID" == 'test-vol-id'
    test "$DHV_PARAMETERS" == '{"key":"val"}'
    test "$DHV_PLUGIN_DIR" == './test_fixtures'
    validate_path "$DHV_HOST_PATH"
    rm -rfv "$2" ;;
  *)
    echo "unknown operation $1"
    exit 1 ;;
esac
