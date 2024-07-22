#!/usr/bin/env bash

# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

# things are prefixed with "Fancy*" because this is a fancy plugin.
# CNI_ARGS='IgnoreUnknown=true;FancyTaskDir=/tmp/cni_args;FancyMessage=hiiii;Another=whatever'
# what we need to do:
# 1. read CNI_ARGS environment variable
#    * write to a file named $FancyTaskDir/victory
# 2. write CNI-spec json to stdout for Nomad to read

# https://github.com/containernetworking/cni/blob/main/SPEC.md#version-success
function version() {
  cat <<EOF
{
    "cniVersion": "1.0.0",
    "supportedVersions": [ "0.1.0", "0.2.0", "0.3.0", "0.3.1", "0.4.0", "1.0.0" ]
}
EOF
}

# https://github.com/containernetworking/cni/blob/main/SPEC.md#add-success
function add() {
  # get our task dir out of the env var
  task_dir="$(echo "$CNI_ARGS" | tr ';' '\n' | awk -F= '/^FancyTaskDir=/ {print$2}')"
  message="$(echo "$CNI_ARGS" | tr ';' '\n' | awk -F= '/^FancyMessage=/ {print$2}')"
  1>&2 echo "got task dir: $task_dir; message: $message"

  mkdir -p "$task_dir"
  # and write something to a file we can check in the test.
  echo "$message" > "$task_dir/victory"
}

# run the appropriate CNI command
case "$CNI_COMMAND" in
  VERSION) version ; exit ;;
  ADD) add ;;
esac

# bogus reply so nomad doesn't error
cat <<EOF
{
    "cniVersion" : "1.0.0",
    "ips": [
        {
          "address": "10.1.0.5/16",
          "gateway": "10.1.0.1",
          "interface": 0
        }
    ],
    "routes": [
      {
        "dst": "0.0.0.0/0"
      }
    ],
    "interfaces": [
        {
            "name": "cni0",
            "mac": "00:11:22:33:44:55"
        }
    ],
    "dns": {
      "nameservers": [ "10.1.0.1" ]
    }
}
EOF

