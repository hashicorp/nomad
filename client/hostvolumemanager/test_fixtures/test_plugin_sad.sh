#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

echo "$1: sad plugin is sad"
echo "$1: it tells you all about it in stderr" 1>&2
exit 1
