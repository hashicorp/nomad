#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


nomad operator api "/v1/allocation/${NOMAD_ALLOC_ID}" | jq '.NetworkStatus.Address | length'
