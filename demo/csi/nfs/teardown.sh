#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


# Clean up all demo components.

set -x

purge() {
  nomad stop -purge "$1"
}

purge web
while true; do
  nomad volume status csi-nfs 2>&1 | grep -E 'No (allocations|volumes)' && break
  sleep 5
done
purge node

nomad volume delete csi-nfs
purge controller

purge nfs

nomad system gc
