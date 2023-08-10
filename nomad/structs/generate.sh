#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -e

FILES="$(ls ./*.go | grep -v -e _test.go -e .generated.go | tr '\n' ' ')"
codecgen \
    -c github.com/hashicorp/go-msgpack/codec \
    -st codec \
    -d 100 \
    -t codegen_generated \
    -o structs.generated.go \
    -nr="(^ACLCache$)|(^IdentityClaims$)" \
    ${FILES}
