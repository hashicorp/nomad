#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -xeuo pipefail

nomad job stop job || true

for _ in {1..5}; do
  sleep 3
  ids="$(nomad volume status -type=host -verbose | awk '/ternal-plugin/ {print$1}')"
  test -z "$ids" && break
  for id in $ids; do 
    nomad volume delete -type=host "$id" || continue
  done
done

