#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
token=$(cat ${1})
if [[ "$token" == "" ]]; then
    echo "no bootstrap token provided!"
    exit 1
fi

while true :
do
    nomad acl bootstrap -token "$token" && break
    echo "waiting for bootstrap"
    sleep 5
done
