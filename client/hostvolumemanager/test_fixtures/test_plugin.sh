#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# plugin for host_volume_plugin_test.go
set -xeuo pipefail

env 1>&2

test "$1" == "$OPERATION"

echo 'all operations should ignore stderr' 1>&2

case $1 in
  create)
    test "$2" == "$HOST_PATH"
    test "$NODE_ID" == 'test-node'
    test "$PARAMETERS" == '{"key":"val"}'
    test "$CAPACITY_MIN_BYTES" -eq 5
    test "$CAPACITY_MAX_BYTES" -eq 10
    mkdir "$2"
    printf '{"path": "%s", "bytes": 5, "context": %s}' "$2" "$PARAMETERS"
    ;;
  delete)
    test "$2" == "$HOST_PATH"
    test "$NODE_ID" == 'test-node'
    test "$PARAMETERS" == '{"key":"val"}'
    rm -rfv "$2" ;;
  version)
    echo '0.0.2' ;;
  *)
    echo "unknown operation $1"
    exit 1 ;;
esac
