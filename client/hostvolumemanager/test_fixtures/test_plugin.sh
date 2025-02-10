#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# plugin for host_volume_plugin_test.go
set -xeuo pipefail

env 1>&2

test "$1" == "$DHV_OPERATION"

echo 'all operations should ignore stderr' 1>&2

case $1 in
  fingerprint)
    echo '{"version": "0.0.2"}' ;;
  create)
    test "$DHV_VOLUME_NAME" == 'test-vol-name'
    test "$DHV_VOLUME_ID" == 'test-vol-id'
    test "$DHV_NAMESPACE" == 'test-namespace'
    test "$DHV_CAPACITY_MIN_BYTES" -eq 5
    test "$DHV_CAPACITY_MAX_BYTES" -eq 10
    test "$DHV_NODE_ID" == 'test-node'
    test "$DHV_NODE_POOL" == 'test-node-pool'
    test "$DHV_PARAMETERS" == '{"key":"val"}'
    test "$DHV_PLUGIN_DIR" == './test_fixtures'
    test -d "$DHV_VOLUMES_DIR"
    target="$DHV_VOLUMES_DIR/$DHV_VOLUME_ID"
    test "$target" != '/'
    mkdir  -p "$target"
    printf '{"path": "%s", "bytes": 5}' "$target"
    ;;
  delete)
    test "$DHV_NODE_ID" == 'test-node'
    test "$DHV_NODE_POOL" == 'test-node-pool'
    test "$DHV_NAMESPACE" == 'test-namespace'
    test "$DHV_VOLUME_NAME" == 'test-vol-name'
    test "$DHV_VOLUME_ID" == 'test-vol-id'
    test "$DHV_PARAMETERS" == '{"key":"val"}'
    test "$DHV_PLUGIN_DIR" == './test_fixtures'
    test -d "$DHV_VOLUMES_DIR"
    target="$DHV_VOLUMES_DIR/$DHV_VOLUME_ID"
    test "$target" != '/'
    test "$DHV_CREATED_PATH" == "$target"
    rm -rfv "$target"
    ;;
  *)
    echo "unknown operation $1"
    exit 1 ;;
esac
