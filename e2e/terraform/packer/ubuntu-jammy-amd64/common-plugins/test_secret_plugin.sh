#!/bin/bash

# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

fingerprint() {
  echo {\"type\": \"secrets\", \"version\": \"0.0.1\"}
}

fetch() {
  # return any passed environment variables as output
  echo '{"result":{'$(printenv | awk -F= '{printf "\"%s\":\"%s\",", $1, $2}' | sed 's/,$//')'}}'
}

case "$1" in
  fingerprint)
    fingerprint
    ;;
  fetch)
    fetch
    ;;
  *)
    exit 1
esac

