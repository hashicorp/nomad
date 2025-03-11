#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

printf '{"error": "%s: sad plugin is sad"}' $1
echo "$1: it tells you all about it in stderr" 1>&2
exit 1
